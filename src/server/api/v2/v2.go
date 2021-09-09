package v2

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/chatterino"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func API(app fiber.Router) fiber.Router {
	api := app.Group("/v2")
	api.Use(cors.New(cors.Config{
		AllowOrigins:  "*",
		ExposeHeaders: "X-Collection-Size,X-Created-ID",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE",
	}))

	Twitch(api)
	rest.RestV2(api)
	gql.GQL(api)
	chatterino.Chatterino(api)

	return api
}
