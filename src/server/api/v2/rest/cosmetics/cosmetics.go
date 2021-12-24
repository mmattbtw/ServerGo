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

		// Retrieve all users of badges
		pipeline := mongo.Pipeline{
			{{
				Key:   "$sort",
				Value: bson.M{"priority": -1},
			}},
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
										"data.selected": true,
										"kind": bson.M{
											"$in": bson.A{"BADGE", "PAINT"},
										},
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
									"$first": bson.M{
										"$filter": bson.M{
											"input": "$items",
											"as":    "it",
											"cond":  bson.M{"$in": bson.A{"$$it.kind", bson.A{"BADGE", "PAINT"}}},
										},
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
								"ent.kind": bson.M{
									"$in": bson.A{"BADGE", "PAINT"},
								},
								"$or": bson.A{
									bson.M{"$expr": bson.M{"$in": bson.A{"$ent.data.role_binding", "$roles.data.ref"}}},
									bson.M{"ent.data.role_binding": bson.M{"$exists": false}},
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
						"$concatArrays": bson.A{"$user_ids", "$entitled._id"},
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
		userCosmetics := []*datastructure.Cosmetic{}
		cur, err := mongo.Collection(mongo.CollectionNameCosmetics).Aggregate(ctx, pipeline)
		if err != nil {
			logrus.WithError(err).Error("mongo, create aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &userCosmetics); err != nil {
			logrus.WithError(err).Error("mongo, execute aggregation")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Find directly assigned users
		result := GetCosmeticsResult{
			Badges: []*restutil.BadgeCosmeticResponse{},
			Paints: []*restutil.PaintCosmeticResponse{},
		}
		badgedUsers := make(map[primitive.ObjectID]bool)
		paintedUsers := make(map[primitive.ObjectID]bool)
		for _, cos := range userCosmetics {
			switch cos.Kind {
			case datastructure.CosmeticKindBadge:
				users := []*datastructure.User{}
				for _, u := range cos.Users {
					if ok := badgedUsers[u.ID]; ok {
						continue
					}
					users = append(users, u)
					badgedUsers[u.ID] = true
				}

				b := restutil.CreateBadgeResponse(cos, users, idType)
				result.Badges = append(result.Badges, b)
			case datastructure.CosmeticKindNametagPaint:
				users := []*datastructure.User{}
				for _, u := range cos.Users {
					if ok := paintedUsers[u.ID]; ok {
						continue
					}
					users = append(users, u)
					paintedUsers[u.ID] = true
				}

				p := restutil.CreatePaintResponse(cos, users, idType)
				result.Paints = append(result.Paints, p)
			}
		}

		b, err := json.Marshal(&result)
		if err != nil {
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		return c.Status(200).Send(b)
	})
}

type GetCosmeticsResult struct {
	Badges []*restutil.BadgeCosmeticResponse `json:"badges"`
	Paints []*restutil.PaintCosmeticResponse `json:"paints"`
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
