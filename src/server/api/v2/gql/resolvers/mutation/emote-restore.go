package mutation_resolvers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (*MutationResolver) RestoreEmote(ctx context.Context, args struct {
	ID     string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	res := mongo.Database.Collection("emotes").FindOne(mongo.Ctx, bson.M{
		"_id":    id,
		"status": datastructure.EmoteStatusDeleted,
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

	_, err = mongo.Database.Collection("emotes").UpdateOne(mongo.Ctx, bson.M{
		"_id": id,
	}, bson.M{
		"$set": bson.M{
			"status":             datastructure.EmoteStatusProcessing,
			"last_modified_date": time.Now(),
		},
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
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
			"status":             datastructure.EmoteStatusLive,
			"last_modified_date": time.Now(),
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeEmoteUndoDelete,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "emotes"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "status", OldValue: emote.Status, NewValue: datastructure.EmoteStatusLive},
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
