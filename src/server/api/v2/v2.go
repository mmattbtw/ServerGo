package v2

import (
	"time"

	"github.com/SevenTV/ServerGo/src/jwt"
	"github.com/SevenTV/ServerGo/src/server/api/v2/chatterino"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest"
	api_websocket "github.com/SevenTV/ServerGo/src/server/api/v2/websocket"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func API(app fiber.Router) fiber.Router {
	api := app.Group("/v2")

	api_websocket.WebSocket(api)
	Twitch(api)
	rest.RestV2(api)
	gql.GQL(api)
	chatterino.Chatterino(api)

	// Debug
	app.Get("/debuguser/:user/:twid", func(c *fiber.Ctx) error {
		var oid primitive.ObjectID
		userID := c.Params("user")
		twid := c.Params("twid")
		if id, err := primitive.ObjectIDFromHex(userID); err == nil {
			oid = id
		}

		authPl := &middleware.PayloadJWT{
			ID:           oid,
			TWID:         twid,
			TokenVersion: "1",
			CreatedAt:    time.Now(),
		}

		tok, err := jwt.Sign(authPl)
		if err != nil {
			return c.Status(401).Send([]byte(err.Error()))
		}

		return c.Status(201).SendString(tok)
	})

	return api
}
