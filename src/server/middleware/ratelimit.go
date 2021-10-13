package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type RateLimiter struct {
	RedisKey string

	Identifier string
	Limit      int32
	Remaining  int32
	Reset      time.Duration
}

func RateLimitMiddleware(tag string, limit int32, duration time.Duration) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		// Get identifier
		// It is one of: Authorized User ID, Client IP Address
		var identifier string
		if c.Locals("user") != nil {
			user := c.Locals("user").(*datastructure.User)
			identifier = user.ID.Hex()
		} else {
			identifier = c.Get("Cf-Connecting-IP") // cf-ip
		}

		if identifier == "" {
			return c.Next()
		}

		// Create hash of identifier + route tag
		h := sha256.New()
		h.Write(utils.S2B(identifier))
		h.Write(utils.S2B(tag))

		// Remaining
		remaining := int64(limit)
		ttl := int64(0)

		// Ensure script
		scriptExists, _ := redis.Client.ScriptExists(c.Context(), redis.RateLimitScriptSHA1).Result()
		if !scriptExists[0] {
			if err := redis.ReloadScripts(); err != nil {
				return err
			}
			logrus.Info("ratelimit, redis: reloaded scripts")
		}

		// Create RateLimiter instance
		redisKey := hex.EncodeToString(h.Sum(nil))
		if result, err := redis.Client.EvalSha(c.Context(), redis.RateLimitScriptSHA1, []string{}, redisKey, duration.Seconds(), limit, 1).Result(); err != nil {
			logrus.WithError(err).Error("ratelimit")
			c.Set("X-RateLimit-Error", err.Error())
			return c.Next()
		} else {
			a := make([]int64, 3)
			for i, v := range result.([]interface{}) {
				val := v.(int64)
				a[i] = val
			}

			remaining -= a[0]
			ttl = a[1]
		}

		// Apply rate limit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(int(limit)))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))

		c.Set("X-RateLimit-Reset", fmt.Sprint(ttl))

		// 429 Too Many Requests?
		if remaining < 1 {
			return c.Status(fiber.StatusTooManyRequests).JSON(&fiber.Map{
				"status": 429,
				"error":  "You are being rate limited",
			})
		}

		return c.Next()
	}
}

// type rateLimitScriptReply struct {
// 	Count int `json:"count"`
// 	TTL   int `json:"ttl"`
// }
