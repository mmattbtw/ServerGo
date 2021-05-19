package middleware

import (
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/jwt"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PayloadJWT struct {
	ID           primitive.ObjectID `json:"id"`          // User App ID
	TWID         string             `json:"twid"`        // Twitch ID
	Permissions  string             `json:"permissions"` // Permission bitmask from user's role
	TokenVersion string             `json:"version"`     // Token version to match against for JWT invalidation
	CreatedAt    time.Time          `json:"created_at"`
}

func UserAuthMiddleware(required bool) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		auth := strings.Split(c.Get("Authorization"), " ")
		if len(auth) != 2 && auth[0] != "Bearer" {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid Token",
			})
		}

		token := strings.Split(auth[1], ".")

		if len(token) != 3 {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid Token",
			})
		}

		pl := &PayloadJWT{}
		if err := jwt.Verify(token, pl); err != nil {
			log.Errorf("jwt, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Invalid Token",
			})
		}

		if pl.CreatedAt.Before(time.Now().Add(-time.Hour * 24 * 60)) {
			if !required {
				return c.Next()
			}
			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "Access Token Expired",
			})
		}

		query := bson.M{
			"_id": pl.ID,
		}

		if pl.TokenVersion == "" {
			query["token_version"] = bson.M{
				"$exists": false,
			}
		} else {
			query["token_version"] = pl.TokenVersion
		}

		res := mongo.Database.Collection("users").FindOne(mongo.Ctx, query)

		err := res.Err()
		if err != nil {
			if err == mongo.ErrNoDocuments {
				if !required {
					return c.Next()
				}
				return c.Status(403).JSON(&fiber.Map{
					"status": 403,
					"error":  "Invalid Token",
				})
			}
			log.Errorf("mongo, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(500).JSON(&fiber.Map{
				"status": 500,
				"error":  "Internal Server Error",
			})
		}

		user := &datastructure.User{}

		err = res.Decode(user)
		if err != nil {
			log.Errorf("mongo, err=%v", err)
			if !required {
				return c.Next()
			}
			return c.Status(500).JSON(&fiber.Map{
				"status": 500,
				"error":  "Internal Server Error",
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
				"error":  "Internal Server Error",
			})
		}

		if err == nil {
			if !required {
				return c.Next()
			}

			return c.Status(403).JSON(&fiber.Map{
				"status": 403,
				"error":  "You Are Banned",
				"reason": reason,
			})
		}

		// Assign role to user
		if user.RoleID != nil {
			role := datastructure.GetRole(mongo.Ctx, user.RoleID)                                               // Try to get the cached role
			user.Role = utils.Ternary(role.ID.IsZero(), datastructure.DefaultRole, &role).(*datastructure.Role) // Assign cached role if available, otherwise set default role
		} else {
			user.Role = datastructure.DefaultRole // If no role assign default role
		}

		c.Locals("user", user)

		return c.Next()
	}
}
