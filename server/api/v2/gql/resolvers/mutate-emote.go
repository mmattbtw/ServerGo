package resolvers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/redis"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/SevenTV/ServerGo/validation"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// Mutate Emote - Edit
//
func (*RootResolver) EditEmote(ctx context.Context, args struct {
	Emote  emoteInput
	Reason *string
}) (*emoteResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	update := bson.M{}
	logChanges := []*mongo.AuditLogChange{}
	req := args.Emote

	if req.Name != nil {
		if !validation.ValidateEmoteName(utils.S2B(*req.Name)) {
			return nil, errInvalidName
		}
		update["name"] = *req.Name
	}
	if req.OwnerID != nil {
		id, err := primitive.ObjectIDFromHex(*req.OwnerID)
		if err != nil {
			return nil, errInvalidOwner
		}
		update["owner"] = id
	}
	if req.Tags != nil {
		tags := *req.Tags
		if len(tags) > 10 {
			return nil, errInvalidTags
		}
		for _, t := range tags {
			if !validation.ValidateEmoteTag(utils.S2B(t)) {
				return nil, errInvalidTag
			}
		}
		update["tags"] = tags
	}
	if req.Visibility != nil {
		i32 := int32(*req.Visibility)
		if !validation.ValidateEmoteVisibility(i32) {
			return nil, errInvalidVisibility
		}
		update["visibility"] = i32
	}

	if len(update) == 0 {
		return nil, errInvalidUpdate
	}

	id, err := primitive.ObjectIDFromHex(req.ID)
	if err != nil {
		return nil, errUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id": id,
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

	if usr.Rank != mongo.UserRankAdmin {
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

	if req.Name != nil {
		if emote.Name != update["name"] {
			logChanges = append(logChanges, &mongo.AuditLogChange{
				Key:      "name",
				OldValue: emote.Name,
				NewValue: update["name"],
			})
		}
	}
	if req.OwnerID != nil {
		if emote.OwnerID != update["owner"] {
			logChanges = append(logChanges, &mongo.AuditLogChange{
				Key:      "owner",
				OldValue: emote.OwnerID,
				NewValue: update["owner"],
			})
		}
	}
	if req.Tags != nil {
		if utils.DifferentArray(emote.Tags, update["tags"].([]string)) {
			logChanges = append(logChanges, &mongo.AuditLogChange{
				Key:      "tags",
				OldValue: emote.Tags,
				NewValue: update["tags"],
			})
		}
	}
	if req.Visibility != nil {
		if emote.Visibility != update["visibility"] {
			logChanges = append(logChanges, &mongo.AuditLogChange{
				Key:      "visibility",
				OldValue: emote.Visibility,
				NewValue: update["visibility"],
			})
		}
	}

	if len(logChanges) > 0 {
		update["last_modified_date"] = time.Now()

		after := options.After
		doc := mongo.Database.Collection("emotes").FindOneAndUpdate(mongo.Ctx, bson.M{
			"_id": id,
		}, bson.M{
			"$set": update,
		}, &options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
		})
		doc.Decode(emote)

		if doc.Err() != nil {
			log.Errorf("mongo, err=%v, id=%s", doc.Err(), id.Hex())
			return nil, errInternalServer
		}

		_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
			Type:      mongo.AuditLogTypeEmoteEdit,
			CreatedBy: usr.ID,
			Target:    &mongo.Target{ID: &id, Type: "emotes"},
			Changes:   logChanges,
			Reason:    args.Reason,
		})

		if err != nil {
			log.Errorf("mongo, err=%v", err)
		}
		return &emoteResolver{
			v: emote,
		}, nil
	}
	return &emoteResolver{
		v: emote,
	}, nil
}

func updateStructFields(struc interface{}) interface{} {
	val := reflect.ValueOf(struc)
	jsonMap := make(map[string]interface{}, val.Type().NumField())

	for i := 0; i < val.Type().NumField(); i++ {
		f := val.Type().Field(i)
		fmt.Println("COCK", f.Name)
	}

	return jsonMap
}

//
// Mutate Emote - Add to Channel
//
func (*RootResolver) AddChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Reason    *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*mongo.User)
	if !ok {
		return nil, errLoginRequired
	}

	emoteID, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, errUnknownEmote
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

	if usr.Rank != mongo.UserRankAdmin {
		if channel.ID.Hex() != usr.ID.Hex() {
			found := false
			for _, e := range channel.EditorIDs {
				if e.Hex() == usr.ID.Hex() {
					found = true
					break
				}
			}
			if !found {
				return nil, errAccessDenied
			}
		}
	}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == emoteID.Hex() {
			return &response{
				Status:  200,
				Message: "no change",
			}, nil
		}
	}

	emoteRes := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id":    emoteID,
		"status": mongo.EmoteStatusLive,
		"$or": bson.A{
			bson.M{
				"visibility": mongo.EmoteVisibilityNormal,
			},
			bson.M{
				"visibility": mongo.EmoteVisibilityPrivate,
				"$or": bson.A{
					bson.M{
						"owner": channelID,
					},
					bson.M{
						"shared_with": channelID,
					},
				},
			},
		},
	})

	emote := &mongo.Emote{}
	err = emoteRes.Err()
	if err == nil {
		err = emoteRes.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errUnknownEmote
		}
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	emoteIDs := append(channel.EmoteIDs, emoteID)
	_, err = mongo.Database.Collection("users").UpdateOne(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": emoteIDs,
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserChannelEmoteAdd,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &channelID, Type: "users"},
		Changes: []*mongo.AuditLogChange{
			{Key: "emotes", OldValue: channel.EmoteIDs, NewValue: emoteIDs},
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
