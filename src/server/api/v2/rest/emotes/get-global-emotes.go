package emotes

import (
	"encoding/json"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetGlobalEmotes(router fiber.Router) {
	router.Get("/global", middleware.RateLimitMiddleware("get-global-emotes", 25, 16*time.Second),
		func(c *fiber.Ctx) error {
			ctx := c.Context()
			c.Set("Cache-Control", "max-age=600")

			var emotes []*datastructure.Emote
			cur, err := mongo.Collection(mongo.CollectionNameEmotes).Find(ctx, bson.M{
				"visibility": bson.M{
					"$bitsAllSet": datastructure.EmoteVisibilityGlobal,
				},
			})
			if err != nil {
				log.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c)
			}
			if err := cur.All(ctx, &emotes); err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			// Find IDs of emote owners
			ownerUserIDMap := make(map[primitive.ObjectID]int)
			ownerIDs := []primitive.ObjectID{}
			for _, emote := range emotes {
				if ownerUserIDMap[emote.OwnerID] == 1 {
					continue
				}

				ownerUserIDMap[emote.OwnerID] = 1
				ownerIDs = append(ownerIDs, emote.OwnerID)
			}

			// Map IDs to struct
			var owners []*datastructure.User
			ownerMap := make(map[primitive.ObjectID]*datastructure.User, len(owners))
			cur, err = mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
				"_id": bson.M{
					"$in": ownerIDs,
				},
			})
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}
			if err := cur.All(ctx, &owners); err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}
			for _, o := range owners {
				ownerMap[o.ID] = o
			}

			response := make([]restutil.EmoteResponse, len(emotes))
			for i, emote := range emotes {
				owner := ownerMap[emote.OwnerID]
				response[i] = restutil.CreateEmoteResponse(emote, owner)
			}

			j, err := json.Marshal(response)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			return c.Send(j)
		})
}
