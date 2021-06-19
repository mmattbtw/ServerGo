package badges

import (
	"encoding/json"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

/*
* Query Params:
* user_identifier: "object_id", "twitch_id", "login"
 */
func GetBadges(router fiber.Router) {
	router.Get("/", func(c *fiber.Ctx) error {
		idType := c.Query("user_identifier")

		if !utils.Contains([]string{"object_id", "twitch_id", "login"}, idType) {
			return restutil.ErrMissingQueryParams().Send(c, `user_identifier: must be 'object_id', 'twitch_id' or 'login'`)
		}

		// Retrieve all badges from the DB
		var badges []*datastructure.Badge
		if err := cache.Find(c.Context(), "badges", "", bson.M{}, &badges); err != nil {
			return err
		}

		// Retrieve all users of badges
		result := GetBadgesResult{
			Badges: []*restutil.BadgeResponse{},
		}
		for _, baj := range badges {
			var users []*datastructure.User
			if err := cache.Find(c.Context(), "users", "", bson.M{
				"_id": bson.M{"$in": baj.Users},
			}, &users); err != nil {
				log.WithError(err).WithField("badge", baj.Name).Errorf("mongo")
				continue
			}

			b := restutil.CreateBadgeResponse(baj, users, idType)
			result.Badges = append(result.Badges, b)
		}

		b, err := json.Marshal(&result)
		if err != nil {
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		return c.Status(501).Send(b)
	})
}

type GetBadgesResult struct {
	Badges []*restutil.BadgeResponse `json:"badges"`
}
