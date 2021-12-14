package cosmetics

import (
	"encoding/json"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
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
		})
		if err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &badges); err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		badgeUserMap := make(map[primitive.ObjectID][]*datastructure.User)

		// Retrieve all users of badges
		badgedUsers := make(map[primitive.ObjectID]bool)
		pipeline := mongo.Pipeline{
			{{
				Key: "$lookup",
				Value: bson.M{
					"from": mongo.CollectionNameEntitlements,
					"let":  bson.M{"item": "$$ROOT"},
					"pipeline": mongo.Pipeline{
						{{
							Key: "$match",
							Value: bson.M{
								"disabled": bson.M{"$not": bson.M{"$eq": true}},
								"$or": bson.A{
									bson.M{
										"kind": "BADGE",
										"$expr": bson.M{
											"$eq": bson.A{"$data.ref", "$$item._id"},
										},
									},
									bson.M{"kind": "ROLE"},
								},
							},
						}},
						{{
							Key: "$group",
							Value: bson.M{
								"_id": "$user_id",
								"items": bson.M{
									"$push": "$$ROOT",
								},
							},
						}},
						{{
							Key: "$set",
							Value: bson.M{
								"ent": bson.M{
									"$arrayElemAt": bson.A{
										"$items",
										bson.M{"$indexOfArray": bson.A{"$items._id", "$$item._id"}},
									},
								},
								"roles": bson.M{
									"$filter": bson.M{
										"input": "$items",
										"as":    "it",
										"cond":  bson.M{"$eq": bson.A{"$$it.kind", "ROLE"}},
									},
								},
							},
						}},
						{{
							Key: "$match",
							Value: bson.M{
								"ent.kind": "BADGE",
								"$expr": bson.M{
									"$in": bson.A{"$ent.data.role_binding", "$roles.data.ref"},
								},
							},
						}},
						{{
							Key:   "$project",
							Value: bson.M{"_id": "$_id"},
						}},
					},
					"as": "entitled",
				},
			}},
			{{
				Key: "$set",
				Value: bson.M{
					"users": bson.M{
						"$concatArrays": bson.A{"$users", "$entitled._id"},
					},
				},
			}},
			{{
				Key:   "$unset",
				Value: bson.A{"entitled"},
			}},

			// Step 4: Add user relation
			{{
				Key: "$lookup",
				Value: bson.M{
					"from":         mongo.CollectionNameUsers,
					"localField":   "users",
					"foreignField": "_id",
					"as":           "user_objects",
				},
			}},
		}
		// Create aggregation
		userCosmetics := []*datastructure.Badge{}
		cur, err = mongo.Collection(mongo.CollectionNameBadges).Aggregate(ctx, pipeline)
		if err != nil {
			logrus.WithError(err).Error("mongo, create aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &userCosmetics); err != nil {
			logrus.WithError(err).Error("mongo, execute aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		for _, baj := range userCosmetics {
			// Map badges
			for _, u := range baj.Users {
				if _, ok := badgedUsers[u.ID]; ok {
					continue
				}

				badgedUsers[u.ID] = true
				badgeUserMap[baj.ID] = append(badgeUserMap[baj.ID], u)
			}
		}

		// Find directly assigned users
		result := GetBadgesResult{
			Badges: []*restutil.BadgeResponse{},
		}
		for _, baj := range badges {
			users := badgeUserMap[baj.ID]

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
	Badges []*cosmeticsResultBadge      `bson:"badges"`
	Roles  []*datastructure.Entitlement `bson:"roles"`
}

type cosmeticsResultBadge struct {
	BadgeID       primitive.ObjectID `bson:"_id"`
	Name          string             `bson:"name"`
	EntitlementID primitive.ObjectID `bson:"ent_id"`
	Priority      int32              `bson:"priority"`
	RoleBinding   primitive.ObjectID `bson:"role_binding"`
}
