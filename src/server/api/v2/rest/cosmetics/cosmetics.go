package cosmetics

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/*
* Query Params:
* user_identifier: "object_id", "twitch_id", "login"
 */
func GetBadges(router fiber.Router) {
	Avatar(router)

	router.Get("/", func(c *fiber.Ctx) error {
		ctx := c.Context()
		c.Set("Cache-Control", "max-age=150 s-maxage=300")

		idType := c.Query("user_identifier")

		if !utils.Contains([]string{"object_id", "twitch_id", "login"}, idType) {
			return restutil.ErrMissingQueryParams().Send(c, `user_identifier: must be 'object_id', 'twitch_id' or 'login'`)
		}

		// Retrieve all badges from the DB
		badges := []*datastructure.Badge{}
		cur, err := mongo.Collection(mongo.CollectionNameBadges).Aggregate(ctx, mongo.Pipeline{
			{{
				Key:   "$sort",
				Value: bson.M{"priority": -1},
			}},
			{{
				Key: "$lookup",
				Value: bson.M{
					"from":         mongo.CollectionNameUsers,
					"localField":   "users",
					"foreignField": "_id",
					"as":           "user_objects",
				},
			}},
		})
		if err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &badges); err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		badgeMap := make(map[primitive.ObjectID][]*datastructure.User)
		for _, badge := range badges {
			badgeMap[badge.ID] = []*datastructure.User{}
		}

		// Retrieve all users of badges
		badgedUsers := make(map[primitive.ObjectID]bool)
		pipeline := mongo.Pipeline{
			// Step 1: Match all users that have this badge
			{{
				Key: "$match",
				Value: bson.M{
					"disabled": bson.M{"$not": bson.M{"$eq": true}},
					"kind": bson.M{
						"$in": bson.A{"ROLE", "BADGE"},
					},
				},
			}},
			// Step 2: Group entitlements by their respective user
			{{
				Key: "$group",
				Value: bson.M{
					"_id": "$user_id",
					"items": bson.M{
						"$push": "$$ROOT",
					},
				},
			}},
			// Step 3: Return the results
			{{
				Key: "$project",
				Value: bson.M{
					// List of roles
					"roles": bson.M{
						"$filter": bson.M{
							"input": "$items",
							"as":    "item",
							"cond":  bson.M{"$eq": bson.A{"$$item.kind", "ROLE"}},
						},
					},
					// List of badges
					"badges": bson.M{
						"$filter": bson.M{
							"input": "$items",
							"as":    "item",
							"cond":  bson.M{"$eq": bson.A{"$$item.kind", "BADGE"}},
						},
					},
				},
			}},
			// Step 4: Add user relation
			{{
				Key: "$lookup",
				Value: bson.M{
					"from":         mongo.CollectionNameUsers,
					"localField":   "_id",
					"foreignField": "_id",
					"as":           "user",
				},
			}},
			{{
				Key: "$set",
				Value: bson.M{
					"user": bson.M{"$first": "$user"},
				},
			}},
		}
		// Create aggregation
		userCosmetics := []*cosmeticsResult{}
		cur, err = mongo.Collection(mongo.CollectionNameEntitlements).Aggregate(ctx, pipeline)
		if err != nil {
			logrus.WithError(err).Error("mongo, create aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &userCosmetics); err != nil {
			logrus.WithError(err).Error("mongo, execute aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		for _, ent := range userCosmetics {
			fmt.Println("hi", ent.Roles)
			// Map badges
			for _, baj := range ent.Badges {
				if _, ok := badgedUsers[baj.UserID]; ok {
					continue
				}

				bb := actions.Entitlements.With(ctx, *baj)
				badge := bb.ReadBadgeData()

				hasRole := false
				for _, rol := range ent.Roles {
					if rol == nil {
						continue
					}
					if badge.RoleBinding == nil {
						hasRole = true
						continue
					}
					rb := actions.Entitlements.With(ctx, *rol)

					if rb.ReadRoleData().ObjectReference == *badge.RoleBinding {
						hasRole = true
						break
					}
				}

				if hasRole {
					badgedUsers[ent.UserID] = true

					badgeMap[badge.ObjectReference] = append(badgeMap[badge.ObjectReference], ent.User)
				}
			}
		}

		// Find directly assigned users
		result := GetBadgesResult{
			Badges: []*restutil.BadgeResponse{},
		}
		for _, baj := range badges {
			users := append(badgeMap[baj.ID], baj.Users...)

			// Find direct users

			b := restutil.CreateBadgeResponse(baj, users, idType)
			result.Badges = append(result.Badges, b)
		}

		b, err := json.Marshal(&result)
		if err != nil {
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		return c.Status(200).Send(b)
	})
}

type GetBadgesResult struct {
	Badges []*restutil.BadgeResponse `json:"badges"`
}

type cosmeticsResult struct {
	UserID primitive.ObjectID           `bson:"_id"`
	User   *datastructure.User          `bson:"user"`
	Badges []*datastructure.Entitlement `bson:"badges"`
	Roles  []*datastructure.Entitlement `bson:"roles"`
}
