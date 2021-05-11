package mutation_resolvers

import (
	"context"
	"time"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/ServerGo/src/validation"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// Mutate Emote - Edit
//
func (*MutationResolver) EditEmote(ctx context.Context, args struct {
	Emote  emoteInput
	Reason *string
}) (*query_resolvers.EmoteResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	update := bson.M{}
	logChanges := []*datastructure.AuditLogChange{}
	req := args.Emote

	if req.Name != nil {
		if !validation.ValidateEmoteName(utils.S2B(*req.Name)) {
			return nil, resolvers.ErrInvalidName
		}
		update["name"] = *req.Name
	}
	if req.OwnerID != nil {
		id, err := primitive.ObjectIDFromHex(*req.OwnerID)
		if err != nil {
			return nil, resolvers.ErrInvalidOwner
		}
		update["owner"] = id
	}
	if req.Tags != nil {
		tags := *req.Tags
		if len(tags) > 10 {
			return nil, resolvers.ErrInvalidTags
		}
		for _, t := range tags {
			if !validation.ValidateEmoteTag(utils.S2B(t)) {
				return nil, resolvers.ErrInvalidTag
			}
		}
		update["tags"] = tags
	}
	if req.Visibility != nil {
		i32 := int32(*req.Visibility)

		update["visibility"] = i32
	}

	if len(update) == 0 {
		return nil, resolvers.ErrInvalidUpdate
	}

	id, err := primitive.ObjectIDFromHex(req.ID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id": id,
	})

	emote := &datastructure.Emote{}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownEmote
		}
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	if !datastructure.UserHasPermission(usr, datastructure.RolePermissionEmoteEditAll) {
		if emote.OwnerID.Hex() != usr.ID.Hex() {
			if err := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
				"_id":     emote.OwnerID,
				"editors": usr.ID,
			}).Err(); err != nil {
				if err == mongo.ErrNoDocuments {
					return nil, resolvers.ErrAccessDenied
				}
				log.Errorf("mongo, err=%v", err)
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	if req.Name != nil {
		if emote.Name != update["name"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "name",
				OldValue: emote.Name,
				NewValue: update["name"],
			})
		}
	}
	if req.OwnerID != nil {
		if emote.OwnerID != update["owner"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "owner",
				OldValue: emote.OwnerID,
				NewValue: update["owner"],
			})
		}
	}
	if req.Tags != nil {
		if utils.DifferentArray(emote.Tags, update["tags"].([]string)) {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "tags",
				OldValue: emote.Tags,
				NewValue: update["tags"],
			})
		}
	}
	if req.Visibility != nil {
		if emote.Visibility != update["visibility"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "visibility",
				OldValue: emote.Visibility,
				NewValue: update["visibility"],
			})
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
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
		if err := doc.Decode(emote); err != nil {
			return nil, err
		}

		if doc.Err() != nil {
			log.Errorf("mongo, err=%v, id=%s", doc.Err(), id.Hex())
			return nil, resolvers.ErrInternalServer
		}

		_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &datastructure.AuditLog{
			Type:      datastructure.AuditLogTypeEmoteEdit,
			CreatedBy: usr.ID,
			Target:    &datastructure.Target{ID: &id, Type: "emotes"},
			Changes:   logChanges,
			Reason:    args.Reason,
		})

		if err != nil {
			log.Errorf("mongo, err=%v", err)
		}

		go discord.SendEmoteEdit(*emote, *usr, logChanges, args.Reason)
		return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
	}

	return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
}
