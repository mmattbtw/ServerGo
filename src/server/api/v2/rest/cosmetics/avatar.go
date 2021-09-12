package cosmetics

import (
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var avatarSizeRegex = regexp.MustCompile("([0-9]{2,3})x([0-9]{2,3})")

func Avatar(router fiber.Router) {
	cacheExpireAt := time.Time{}
	avatarMap := sync.Map{}
	resetCache := func(key, value interface{}) bool {
		avatarMap.Delete(key)
		return true
	}

	hashURL := func(u string) string {
		u = avatarSizeRegex.ReplaceAllString(u, "300x300")
		hasher := sha256.New()
		hasher.Write(utils.S2B(u))
		return hex.EncodeToString(hasher.Sum(nil))
	}

	router.Get("/avatars", func(c *fiber.Ctx) error {
		ctx := c.Context()

		var users []*datastructure.User
		cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
			"avatar_url": bson.M{"$exists": true},
		}, options.Find().SetProjection(bson.M{
			"profile_image_url": 1,
			"avatar_url":        1,
		}))
		if err != nil {
			log.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}
		if err = cur.All(ctx, &users); err != nil {
			log.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c, err.Error())
		}

		result := make(map[string]string, len(users))
		for _, u := range users {
			h := hashURL(u.ProfileImageURL)
			result[h] = u.CustomAvatarURL
		}

		c.Set("Cache-Control", "max-age=600")
		b, _ := json.Marshal(result)
		return c.Send(b)
	})

	router.Get("/avatar-map/twitch", func(c *fiber.Ctx) error {
		ctx := c.Context()
		url := c.Query("url")
		if url == "" {
			return restutil.ErrBadRequest().Send(c, "url query parameter is required")
		}
		hash := hashURL(url)

		if cacheExpireAt.IsZero() || cacheExpireAt.Unix() < time.Now().Unix() {
			log.Info("cache reset")
			// Find users with custom avatars
			var users []*datastructure.User
			cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
				"avatar_url": bson.M{"$exists": true},
			}, options.Find().SetProjection(bson.M{
				"profile_image_url": 1,
				"avatar_url":        1,
			}))
			if err != nil {
				log.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c, err.Error())
			}
			if err = cur.All(ctx, &users); err != nil {
				log.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c, err.Error())
			}
			avatarMap.Range(resetCache)

			// Construct a map of profile image urls to custom avatar url
			for _, u := range users {
				if u.CustomAvatarURL == "" {
					continue
				}

				avh := hashURL(u.ProfileImageURL)
				avatarMap.Store(avh, u.CustomAvatarURL)
			}
			cacheExpireAt = time.Now().Add(time.Minute * 1)
		}

		// Get UUID from twitch url
		var responseUrl string

		customURL, ok := avatarMap.Load(hash)
		if !ok {
			responseUrl = "https://cdn.7tv.app/emote/60bcb44f7229037ee386d1ab/4x"
		} else {
			responseUrl = customURL.(string)
		}

		b, _ := json.Marshal(map[string]string{
			"url":      responseUrl,
			"cache_id": hash,
		})
		return c.Send(b)
	})
}
