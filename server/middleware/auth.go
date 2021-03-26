package middleware

import (
	"strings"

	"github.com/SevenTV/ServerGo/jwt"
	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/redis"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func UserAuthMiddleware(required bool) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		auth := strings.Split(c.Get("Authorization"), " ")
		if len(auth) != 2 && auth[0] != "Bearer" {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid token.1",
			})
		}

		token := strings.Split(auth[1], ".")

		if len(token) != 3 {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid token.2",
			})
		}

		pl, err := jwt.Verify(token)
		if err != nil {
			log.Errorf("jwt, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid token.3",
			})
		}

		val := utils.B2S(utils.S2B(pl.ID))

		id, err := primitive.ObjectIDFromHex(val)
		if err != nil {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid token.4",
			})
		}

		res := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
			"_id": id,
		})

		err = res.Err()
		if err != nil {
			if err == mongo.ErrNoDocuments {
				if !required {
					return c.Next()
				}
				return c.Status(403).JSON(&fiber.Map{
					"status": 403,
					"error":  "Invalid token.5",
				})
			}
			log.Errorf("mongo, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(500).JSON(&fiber.Map{
				"status": 500,
				"error":  "Internal server error.",
			})
		}

		user := &mongo.User{}

		err = res.Decode(user)
		if err != nil {
			log.Errorf("mongo, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(500).JSON(&fiber.Map{
				"status": 500,
				"error":  "Internal server error.",
			})
		}

		reason, err := redis.Client.HGet(redis.Ctx, "user:bans", user.ID.Hex()).Result()
		if err != nil && err != redis.ErrNil {
			log.Errorf("redis, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(500).JSON(&fiber.Map{
				"status": 500,
				"error":  "Internal server error.",
			})
		}

		if err == nil {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "You are banned.",
				"reason": reason,
			})
		}

		c.Locals("user", user)

		return c.Next()
	}
}
