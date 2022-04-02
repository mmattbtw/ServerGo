package cosmetics

import (
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

var avatarSizeRegex = regexp.MustCompile("([0-9]{2,3})x([0-9]{2,3})")

func Avatar(router fiber.Router) {
	hashURL := func(u string) string {
		u = avatarSizeRegex.ReplaceAllString(u, "300x300")
		hasher := sha256.New()
		hasher.Write(utils.S2B(u))
		return hex.EncodeToString(hasher.Sum(nil))
	}

	router.Get("/cosmetics/avatars", func(c *fiber.Ctx) error {
		ctx := c.Context()
		mapTo := c.Query("map_to", "hash") // Retrieve key mapping parameter

		// Let's do a little bit of fetching data
		var users []*datastructure.User
		pipeline := mongo.Pipeline{
			// Step 1: Match all users with a set profile picture
			bson.D{bson.E{
				Key: "$match",
				Value: bson.M{
					"profile_picture_id": bson.M{"$exists": true},
				},
			}},
			// Step 2: Add "user" to document root as the user document
			bson.D{bson.E{
				Key:   "$addFields",
				Value: bson.M{"user": "$$ROOT"},
			}},

			// Step 3: Add ROLE entitlementd of each user to our document
			// This is used to check permissions. (i.e can the user have a custom avatar)
			bson.D{bson.E{
				Key: "$lookup",
				Value: bson.M{
					"from": "entitlements",
					"let":  bson.M{"user_id": "$_id"},
					"pipeline": mongo.Pipeline{
						bson.D{bson.E{
							Key: "$match",
							Value: bson.M{
								"disabled": bson.M{"$not": bson.M{"$eq": true}}, // here we make sure the entitlement is active
								"kind":     "ROLE",
								"$expr": bson.M{
									"$eq": bson.A{"$user_id", "$$user_id"},
								},
							},
						}},
					},
					"as": "entitled_roles", // output to entitled_roles
				},
			}},
		}
		cur, err := mongo.Collection(mongo.CollectionNameUsers).Aggregate(ctx, pipeline)
		if err != nil {
			logrus.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		// Iterate and append eligible users to the response result
		for {
			if ok := cur.Next(ctx); !ok {
				break
			}

			var u *avatarsPipelineResult
			if err = cur.Decode(&u); err != nil {
				logrus.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c)
			}
			if strings.HasPrefix(u.User.ProfileImageURL, "https://static-cdn.jtvnw.net/user-default-pictures-uv") {
				continue
			}

			// Ensure permissions
			hasPermission := false
			for _, ent := range u.EntitledRoles {
				rb := actions.Entitlements.With(ctx, *ent)
				roleID := rb.ReadRoleData().ObjectReference
				role := datastructure.GetRole(&roleID)

				// Check: user has "administrator", or "use custom avatars" permission
				if utils.BitField.HasBits(role.Allowed, datastructure.RolePermissionAdministrator) || utils.BitField.HasBits(role.Allowed, datastructure.RolePermissionUseCustomAvatars) {
					hasPermission = true
				}
			}
			// If no permission from entitled role, also check the user's directly assigned role
			if !hasPermission && u.User.RoleID != nil {
				role := datastructure.GetRole(u.User.RoleID)
				// Check: user has "administrator", or "use custom avatars" permission
				if utils.BitField.HasBits(role.Allowed, datastructure.RolePermissionAdministrator) || utils.BitField.HasBits(role.Allowed, datastructure.RolePermissionUseCustomAvatars) {
					hasPermission = true
				}
			}
			if hasPermission {
				users = append(users, u.User)
			}
		}

		// Create response
		result := make(map[string]string, len(users))
		for _, u := range users {
			var key string
			switch mapTo {
			case "hash":
				key = hashURL(u.ProfileImageURL)
			case "twitch_id":
				key = u.TwitchID
			case "object_id":
				key = u.ID.Hex()
			case "login":
				key = u.Login
			}
			if key == "" {
				continue
			}

			result[key] = datastructure.UserUtil.GetProfilePictureURL(u)
		}

		c.Set("Cache-Control", "max-age=600")
		b, _ := json.Marshal(result)
		return c.Send(b)
	})
}

type avatarsPipelineResult struct {
	*datastructure.User `bson:"user"`
	EntitledRoles       []*datastructure.Entitlement `bson:"entitled_roles"`
}
