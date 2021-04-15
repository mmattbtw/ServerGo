package v2

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/emotes"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql"
	"github.com/gofiber/fiber/v2"
)

func API(app fiber.Router) fiber.Router {
	api := app.Group("/v2")

	Twitch(api)
	emotes.Emotes(api)
	gql.GQL(api)

	return api
}
