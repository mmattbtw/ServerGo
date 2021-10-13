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
		c.Set("Cache-Control", "max-age=300")

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
			var users []*datastructure.User
			// Find directly assigned users
			cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
				"_id": bson.M{"$in": baj.Users},
			})
			if err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo")
				continue
			}
			if err = cur.All(ctx, &users); err != nil {
				logrus.WithError(err).WithField("badge", baj.Name).Error("mongo")
				continue
			}

			// Find entitled users
			builders, err := actions.Entitlements.FetchEntitlements(ctx, struct {
				Kind            *datastructure.EntitlementKind
				ObjectReference primitive.ObjectID
			}{
				Kind:            &datastructure.EntitlementKindBadge,
				ObjectReference: baj.ID,
			})
			if err != nil {
				logrus.WithError(err).Error("GetBadges, FetchEntitlements")
			}
			for _, eb := range builders {
				data := eb.ReadBadgeData()
				ok := false
				if data.RoleBinding != nil {
					// Badge has role binding, we will now ensure user can actually use this badge
					if eb.User.RoleID == data.RoleBinding {
						ok = true
					} else { // The user doesn't have the role bound directly, so we will check for an entitled role
						ub, err := actions.Users.With(ctx, eb.User)
						if err != nil {
							logrus.WithError(err).WithField("badge", baj.Name).Error("actions")
							continue
						}

						uents, err := ub.FetchEntitlements(&datastructure.EntitlementKindRole)
						if err != nil {
							logrus.WithError(err).WithField("badge", baj.Name).Error("actions")
						}
						// Iterate role entitlements for the user
						for _, uent := range uents {
							role := uent.ReadRoleData()
							if role.ObjectReference != *data.RoleBinding {
								continue
							}
							ok = true
						}
					}
				} else {
					ok = true
				}
				if !ok { // No permission to use this badge. Unlucky
					continue
				}

				users = append(users, eb.User)
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
