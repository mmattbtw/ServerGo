package cosmetics

import (
	"encoding/json"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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
		var badges []*datastructure.Badge
		cur, err := mongo.Collection(mongo.CollectionNameBadges).Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"_id": -1}))
		if err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &badges); err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Retrieve all users of badges
		result := GetBadgesResult{
			Badges: []*restutil.BadgeResponse{},
		}
		for _, baj := range badges {
			//
			pipeline := mongo.Pipeline{
				// Step 1: Match all users that have this badge
				bson.D{{
					Key: "$match",
					Value: bson.M{
						"kind":     "BADGE",
						"data.ref": baj.ID,
					},
				}},
				bson.D{{
					Key:   "$addFields",
					Value: bson.M{"badge": "$$ROOT"},
				}},

				// Step 2: Add role bindings
				bson.D{{
					Key: "$lookup",
					Value: bson.M{
						"from": "entitlements",
						"let":  bson.M{"user_id": "$user_id"},
						"pipeline": mongo.Pipeline{
							bson.D{{
								Key: "$match",
								Value: bson.M{
									"disabled": bson.M{"$not": bson.M{"$eq": true}},
									"kind":     "ROLE",
									"$expr": bson.M{
										"$eq": bson.A{"$user_id", "$$user_id"},
									},
								},
							}},
						},
						"as": "role_bindings",
					},
				}},
			}
			// Create aggregation
			var ents []*badgesAggregationResult
			cur, err := mongo.Collection(mongo.CollectionNameEntitlements).Aggregate(ctx, pipeline)
			if err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo, create aggregation")
				continue
			}
			if err = cur.All(ctx, &ents); err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo, execute aggregation")
				continue
			}
			var userIDs []primitive.ObjectID
			for _, ent := range ents {
				bb := actions.Entitlements.With(ctx, *ent.Badge)
				badge := bb.ReadBadgeData()

				hasRole := false
				for _, r := range ent.RoleBindings {
					rb := actions.Entitlements.With(ctx, *r)

					if rb.ReadRoleData().ObjectReference == *badge.RoleBinding {
						hasRole = true
						break
					}
				}

				if hasRole {
					userIDs = append(userIDs, ent.Badge.UserID)
				}
			}

			// Find directly assigned users
			userIDs = append(userIDs, baj.Users...)

			var users []*datastructure.User
			cur, err = mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
				"_id": bson.M{"$in": userIDs},
			})
			if err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo")
				continue
			}
			if err = cur.All(ctx, &users); err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo")
				continue
			}

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

type badgesAggregationResult struct {
	Badge        *datastructure.Entitlement   `bson:"badge"`
	RoleBindings []*datastructure.Entitlement `bson:"role_bindings"`
}
