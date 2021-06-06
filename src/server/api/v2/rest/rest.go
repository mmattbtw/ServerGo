package rest

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/emotes"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/users"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func RestV2(app fiber.Router) fiber.Router {
	restGroup := app.Group("/")
	restGroup.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")

		return c.Next()
	})
	restGroup.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE",
	}))

	emoteGroup := restGroup.Group("/emotes")
	emotes.CreateEmoteRoute(emoteGroup)
	emotes.GetGlobalEmotes(emoteGroup)
	emotes.GetEmoteRoute(emoteGroup)

	userGroup := restGroup.Group("/users")
	users.GetUser(userGroup)
	users.GetChannelEmotesRoute(userGroup)

	return nil
}
