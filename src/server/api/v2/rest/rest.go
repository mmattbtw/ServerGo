package rest

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/emotes"
	"github.com/gofiber/fiber/v2"
)

func RestV2(app fiber.Router) fiber.Router {
	restGroup := app.Group("/")
	restGroup.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")

		return c.Next()
	})

	emoteGroup := restGroup.Group("/emotes")
	emotes.CreateEmoteRoute(emoteGroup)
	emotes.GetEmoteRoute(emoteGroup)
	emotes.GetChannelEmotesRoute(emoteGroup)

	return nil
}
