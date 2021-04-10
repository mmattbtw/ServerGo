package resolvers

import (
	"context"
	"time"

	"github.com/SevenTV/ServerGo/mongo"
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

	field, failed := GenerateSelectedFieldMap(ctx, maxDepth)
	if failed {
		return nil, errDepth
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
		return GenerateEmoteResolver(ctx, emote, &emote.ID, field.children)
	}
	return GenerateEmoteResolver(ctx, emote, &emote.ID, field.children)
}
