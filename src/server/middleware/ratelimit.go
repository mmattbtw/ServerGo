package middleware

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
)

type RateLimiter struct {
	request  *fiber.Ctx
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
		} else if len(c.IPs()) > 0 {
			identifier = c.IPs()[0]
		} else {
			identifier = c.Context().RemoteAddr().String()
		}

		// Create hash of identifier + route tag
		h := sha1.New()
		h.Write(utils.S2B(identifier))
		h.Write(utils.S2B(tag))

		// Create RateLimiter instance
		redisKey := fmt.Sprintf("rl:%s", hex.EncodeToString(h.Sum(nil)))
		fmt.Println(redisKey)
		rl := RateLimiter{
			c,          // Connection
			redisKey,   // Redis Key
			identifier, // Identifier
			limit,      // Limit
			limit - 1,  // Remaining
			duration,   // Reset After
		}
		rl.CheckLimit()

		// Apply rate limit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(int(rl.Limit)))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(int(rl.Remaining)))

		resetAt, _ := redis.Client.HGet(redis.Ctx, rl.RedisKey, "reset").Time()
		resetIn := duration.Seconds() - time.Now().Sub(resetAt).Seconds() // Calculate seconds until reset
		c.Set("X-RateLimit-Reset", strconv.Itoa(int(resetIn)))

		// 429 Too Many Requests?
		if rl.Remaining < 1 {
			return c.Status(fiber.StatusTooManyRequests).JSON(&fiber.Map{
				"status": 429,
				"error":  "You are being rate limited",
			})
		}

		return c.Next()
	}
}

func (rl *RateLimiter) CheckLimit() bool {
	if !redis.Client.HExists(redis.Ctx, rl.RedisKey, "remaining").Val() {
		resetAt := time.Now()
		resetAt.Add(rl.Reset)

		redis.Client.HSet(redis.Ctx, rl.RedisKey,
			"identifier", rl.Identifier,
			"limit", rl.Limit,
			"remaining", rl.Limit-1,
			"reset", resetAt,
		)
		redis.Client.Expire(redis.Ctx, rl.RedisKey, rl.Reset)
	} else {
		rl.Remaining = int32(redis.Client.HIncrBy(redis.Ctx, rl.RedisKey, "remaining", -1).Val())
	}

	return false
}
