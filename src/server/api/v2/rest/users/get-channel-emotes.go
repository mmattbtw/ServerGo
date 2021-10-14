package users

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetChannelEmotesRoute(router fiber.Router) {
	router.Get("/:user/emotes", middleware.RateLimitMiddleware("get-user-emotes", 100, 9*time.Second),
		func(c *fiber.Ctx) error {
			ctx := c.Context()
			channelIdentifier := c.Params("user")
			c.Set("Cache-Control", "max-age=30")

			// Find channel user
			var channel *datastructure.User
			ub, err := actions.Users.Get(ctx, bson.M{
				"$or": bson.A{
					bson.M{"id": channelIdentifier},
					bson.M{"login": strings.ToLower(channelIdentifier)},
					bson.M{"yt_id": channelIdentifier},
				},
			})
			if err != nil {
				return restutil.ErrUnknownUser().Send(c, err.Error())
			}
			if ub.IsBanned() {
				return c.SendString("[]")
			}
			channel = &ub.User

			// Build query for emotes
			var emotes []*datastructure.Emote
			emoteFilter := bson.M{
				"_id": bson.M{
					"$in": channel.EmoteIDs,
				},
			}
			if !channel.HasPermission(datastructure.RolePermissionUseZeroWidthEmote) {
				// Omit zerowidth emote if the user lacks permission to use those
				emoteFilter["visibility"] = bson.M{
					"$bitsAllClear": datastructure.EmoteVisibilityZeroWidth,
				}
			}

			// Find emotes
			cur, err := mongo.Collection(mongo.CollectionNameEmotes).Find(ctx, emoteFilter)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}
			if err := cur.All(ctx, &emotes); err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			// Find aliases and replace
			channel.Emotes = &emotes
			emotes = datastructure.UserUtil.GetAliasedEmotes(channel)

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

			// Create final response
			response := make([]restutil.EmoteResponse, len(emotes))
			for i, emote := range emotes {
				response[i] = restutil.CreateEmoteResponse(emote, ownerMap[emote.OwnerID])
			}

			j, err := json.Marshal(response)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			return c.Send(j)
		})
}
