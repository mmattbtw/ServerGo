package api

import (
	apiv2 "github.com/SevenTV/ServerGo/server/api/v2"
	"github.com/gofiber/fiber/v2"
)

func API(app fiber.Router) fiber.Router {
	api := app.Group("/api")

	apiv2.API(api)

	return api
}
