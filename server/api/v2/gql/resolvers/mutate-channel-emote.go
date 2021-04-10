package resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/redis"
	"github.com/SevenTV/ServerGo/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// Mutate Emote - Add to Channel
//
func (*RootResolver) AddChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Reason    *string
}) (*userResolver, error) {
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

	if !mongo.UserHasPermission(usr, mongo.RolePermissionManageUsers) {
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

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == emoteID.Hex() {
			return GenerateUserResolver(ctx, channel, &channelID, field.children)
		}
	}

	emoteRes := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id":    emoteID,
		"status": mongo.EmoteStatusLive,
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

	sharedWith := []string{}
	for _, v := range emote.SharedWith {
		sharedWith = append(sharedWith, v.Hex())
	}
	if utils.HasBits(int64(emote.Visibility), int64(mongo.EmoteVisibilityPrivate)) {
		if emote.OwnerID.Hex() != channelID.Hex() || !utils.Contains(sharedWith, emoteID.Hex()) {
			return nil, errUnknownEmote
		}
	}

	emoteIDs := append(channel.EmoteIDs, emoteID)
	after := options.After
	doc := mongo.Database.Collection("users").FindOneAndUpdate(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": emoteIDs,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(channel); err != nil {
		return nil, err
	}

	if doc.Err() != nil {
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

	return GenerateUserResolver(ctx, channel, &channelID, field.children)
}

//
// Mutate Emote - Remove from Channel
//
func (*RootResolver) RemoveChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Reason    *string
}) (*userResolver, error) {
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

	if !mongo.UserHasPermission(usr, mongo.RolePermissionManageUsers) {
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

	found := false

	newIds := []primitive.ObjectID{}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == emoteID.Hex() {
			found = true
		} else {
			newIds = append(newIds, eID)
		}
	}

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
	}

	if !found {
		return GenerateUserResolver(ctx, channel, &channelID, field.children)
	}

	_, err = mongo.Database.Collection("users").UpdateOne(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": newIds,
		},
	})
	after := options.After
	doc := mongo.Database.Collection("users").FindOneAndUpdate(mongo.Ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": newIds,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(channel); err != nil {
		return nil, err
	}

	if doc.Err() != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, errInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &mongo.AuditLog{
		Type:      mongo.AuditLogTypeUserChannelEmoteRemove,
		CreatedBy: usr.ID,
		Target:    &mongo.Target{ID: &channelID, Type: "users"},
		Changes: []*mongo.AuditLogChange{
			{Key: "emotes", OldValue: channel.EmoteIDs, NewValue: newIds},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return GenerateUserResolver(ctx, channel, &channelID, field.children)
}
