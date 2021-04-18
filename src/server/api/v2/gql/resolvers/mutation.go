package resolvers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type response struct {
	Status  int32  `json:"status"`
	Message string `json:"message"`
}

type emoteInput struct {
	ID         string    `json:"id"`
	Name       *string   `json:"name"`
	OwnerID    *string   `json:"owner_id"`
	Visibility *int32    `json:"visibility"`
	Tags       *[]string `json:"tags"`
}

var (
	errInvalidName    = fmt.Errorf("the new name is not valid")
	errLoginRequired  = fmt.Errorf("you need to be logged in to do that")
	errInvalidOwner   = fmt.Errorf("the new owner ID is not valid")
	errInvalidTags    = fmt.Errorf("you specified more than 10 tags")
	errInvalidTag     = fmt.Errorf("some of your tags are invalid, please check them")
	errInvalidUpdate  = fmt.Errorf("you have to supply an update")
	errUnknownEmote   = fmt.Errorf("an emote with that ID does not exist")
	errUnknownChannel = fmt.Errorf("a channel with that ID does not exist")
	errUnknownUser    = fmt.Errorf("a user with that ID does not exist")
	errAccessDenied   = fmt.Errorf("you don't have permission to do that")
	errChannelBanned  = fmt.Errorf("that channel is currently banned")
	errUserBanned     = fmt.Errorf("that user is currently banned")
	errUserNotBanned  = fmt.Errorf("that user is currently not banned")
	errYourself       = fmt.Errorf("you cannot do that to yourself")
	errNoReason       = fmt.Errorf("you must provide a reason")
)

func (*RootResolver) ReportEmote(ctx context.Context, args struct {
	EmoteID string
	Reason  *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, errUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id":    id,
		"status": mongo.EmoteStatusLive,
	})

	emote := &mongo.Emote{}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownEmote
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Database.Collection("reports").UpdateOne(mongo.Ctx, bson.M{
		"target.id":   emote.ID,
		"target.type": "emotes",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) ReportUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, errUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, errYourself
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	if err == nil {
		return nil, errChannelBanned
	}

	res := mongo.Database.Collection("user").FindOne(mongo.Ctx, bson.M{
		"_id": id,
	})

	user := &mongo.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(user)
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownUser
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Database.Collection("reports").UpdateOne(mongo.Ctx, bson.M{
		"target.id":   user.ID,
		"target.type": "users",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   user.ID,
			"target.type": "users",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) BanUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionBanUsers) {
		return nil, errAccessDenied
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, errUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, errYourself
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	if err == nil {
		return nil, errChannelBanned
	}

	res := mongo.Database.Collection("user").FindOne(mongo.Ctx, bson.M{
		"_id": id,
	})

	user := &mongo.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(user)
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownUser
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if user.Role.Position >= usr.Role.Position {
		return nil, errAccessDenied
	}

	reasonN := "Not Provided"
	if args.Reason != nil {
		reasonN = *args.Reason
	}

	ban := &mongo.Ban{
		UserID:     &user.ID,
		Active:     true,
		Reason:     reasonN,
		IssuedByID: &usr.ID,
	}

	_, err = mongo.Database.Collection("bans").InsertOne(mongo.Ctx, ban)
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = redis.Client.HSet(redis.Ctx, "user:bans", id.Hex(), reasonN).Result()
	if err != nil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserBan,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "users"},
		Changes:   nil,
		Reason:    args.Reason,
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) UnbanUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionBanUsers) {
		return nil, errAccessDenied
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, errUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, errYourself
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil {
		if err != redis.ErrNil {
			return nil, errUserNotBanned
		}
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	res := mongo.Database.Collection("user").FindOne(mongo.Ctx, bson.M{
		"_id": id,
	})

	user := &mongo.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(user)
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownUser
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if user.Role.Position >= usr.Role.Position {
		return nil, errAccessDenied
	}

	_, err = mongo.Database.Collection("bans").UpdateMany(mongo.Ctx, bson.M{
		"user_id": user.ID,
		"active":  true,
	}, bson.M{
		"$set": bson.M{
			"active": false,
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = redis.Client.HDel(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserUnban,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "users"},
		Changes:   nil,
		Reason:    args.Reason,
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) DeleteEmote(ctx context.Context, args struct {
	ID     string
	Reason string
}) (*bool, error) {
	if args.Reason == "" {
		return nil, errNoReason
	}

	var success bool

	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, errUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id": id,
		"status": bson.M{
			"$ne": mongo.EmoteStatusDeleted,
		},
	})

	emote := &mongo.Emote{}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownEmote
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionEmoteEditAll) {
		if emote.OwnerID.Hex() != usr.ID.Hex() {
			if err := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
				"_id":     emote.OwnerID,
				"editors": usr.ID,
			}).Err(); err != nil {
				if err == mongo.ErrNoDocuments {
					return nil, errAccessDenied
				}
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
		}
	}

	_, err = mongo.Database.Collection("emotes").UpdateOne(mongo.Ctx, bson.M{
		"_id": id,
	}, bson.M{
		"$set": bson.M{
			"status":             mongo.EmoteStatusDeleted,
			"last_modified_date": time.Now(),
		},
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeEmoteDelete,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "emotes"},
		Changes: []*mongo.AuditLogChange{
			{Key: "status", OldValue: emote.Status, NewValue: mongo.EmoteStatusDeleted},
		},
		Reason: &args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(4)

	for i := 1; i <= 4; i++ {
		go func(i int) {
			defer wg.Done()
			obj := fmt.Sprintf("emote/%s", emote.ID.Hex())
			err := aws.Expire(configure.Config.GetString("aws_cdn_bucket"), obj, i)
			if err != nil {
				log.Errorf("aws, err=%v, obj=%s", err, obj)
			}
		}(i)
	}

	_, err = mongo.Database.Collection("users").UpdateMany(mongo.Ctx, bson.M{
		"emotes": id,
	}, bson.M{
		"$pull": bson.M{
			"emotes": id,
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	wg.Wait()

	discord.SendEmoteDelete(*emote, *usr, args.Reason)
	success = true
	return &success, nil
}

func (*RootResolver) RestoreEmote(ctx context.Context, args struct {
	ID     string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, errUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id":    id,
		"status": mongo.EmoteStatusDeleted,
	})

	emote := &mongo.Emote{}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownEmote
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionEmoteEditAll) {
		if emote.OwnerID.Hex() != usr.ID.Hex() {
			if err := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
				"_id":     emote.OwnerID,
				"editors": usr.ID,
			}).Err(); err != nil {
				if err == mongo.ErrNoDocuments {
					return nil, errAccessDenied
				}
				log.Errorf("mongo, err=%v", err)
				return nil, errInternalServer
			}
		}
	}

	_, err = mongo.Database.Collection("emotes").UpdateOne(mongo.Ctx, bson.M{
		"_id": id,
	}, bson.M{
		"$set": bson.M{
			"status":             mongo.EmoteStatusProcessing,
			"last_modified_date": time.Now(),
		},
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	wg := &sync.WaitGroup{}
	wg.Add(4)

	for i := 1; i <= 4; i++ {
		go func(i int) {
			defer wg.Done()
			obj := fmt.Sprintf("emote/%s", emote.ID.Hex())
			err := aws.Unexpire(configure.Config.GetString("aws_cdn_bucket"), obj, i)
			if err != nil {
				log.Errorf("aws, err=%v, obj=%s", err, obj)
			}
		}(i)
	}

	wg.Wait()

	_, err = mongo.Database.Collection("emotes").UpdateOne(mongo.Ctx, bson.M{
		"_id": id,
	}, bson.M{
		"$set": bson.M{
			"status":             mongo.EmoteStatusLive,
			"last_modified_date": time.Now(),
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeEmoteUndoDelete,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &id, Type: "emotes"},
		Changes: []*mongo.AuditLogChange{
			{Key: "status", OldValue: emote.Status, NewValue: mongo.EmoteStatusLive},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) AddChannelEditor(ctx context.Context, args struct {
	ChannelID string
	EditorID  string
	Reason    *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	editorID, err := primitive.ObjectIDFromHex(args.EditorID)
	if err != nil {
		return nil, errUnknownUser
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, errUnknownChannel
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	if err == nil {
		return nil, errChannelBanned
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", editorID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	if err == nil {
		return nil, errUserBanned
	}

	res := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
		"_id": channelID,
	})

	channel := &mongo.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(channel)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownChannel
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionManageUsers) {
		if channel.ID.Hex() != usr.ID.Hex() {
			return nil, errAccessDenied
		}
	}

	for _, eID := range channel.EditorIDs {
		if eID.Hex() == editorID.Hex() {
			return &response{
				Status:  200,
				Message: "no change",
			}, nil
		}
	}

	editorIDs := append(channel.EditorIDs, editorID)
	_, err = mongo.Database.Collection("users").UpdateOne(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"editors": editorIDs,
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserChannelEditorAdd,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &channelID, Type: "users"},
		Changes: []*mongo.AuditLogChange{
			{Key: "editors", OldValue: channel.EditorIDs, NewValue: editorIDs},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

func (*RootResolver) RemoveChannelEditor(ctx context.Context, args struct {
	ChannelID string
	EditorID  string
	Reason    *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	editorID, err := primitive.ObjectIDFromHex(args.EditorID)
	if err != nil {
		return nil, errUnknownUser
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, errUnknownChannel
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, errInternalServer
	}

	if err == nil {
		return nil, errChannelBanned
	}

	res := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
		"_id": channelID,
	})

	channel := &mongo.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(channel)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownChannel
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	if !mongo.UserHasPermission(usr, mongo.RolePermissionManageUsers) {
		if channel.ID.Hex() != usr.ID.Hex() {
			return nil, errAccessDenied
		}
	}

	found := false

	newIds := []primitive.ObjectID{}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == editorID.Hex() {
			found = true
		} else {
			newIds = append(newIds, eID)
		}
	}

	if !found {
		return &response{
			Status:  200,
			Message: "no change",
		}, nil
	}

	_, err = mongo.Database.Collection("users").UpdateOne(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"editors": newIds,
		},
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserChannelEditorRemove,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &channelID, Type: "users"},
		Changes: []*mongo.AuditLogChange{
			{Key: "editors", OldValue: channel.EditorIDs, NewValue: newIds},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}
