package rest

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/emotes"
	"github.com/gofiber/fiber/v2"
)

func RestV2(app fiber.Router) fiber.Router {

	emoteGroup := app.Group("/emotes")
	emotes.CreateEmoteRoute(emoteGroup)

	return nil
}
