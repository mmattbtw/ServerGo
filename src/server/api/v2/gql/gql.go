package gql

import (
	"context"

	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gobuffalo/packr/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/graph-gophers/graphql-go"

	log "github.com/sirupsen/logrus"
)

type GQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operation_name"`
}

func GQL(app fiber.Router) fiber.Router {
	gql := app.Group("/gql", middleware.UserAuthMiddleware(false))

	box := packr.New("gql", "./scheme")

	s, err := box.FindString("scheme.gql")

	if err != nil {
		panic(err)
	}

	schema := graphql.MustParseSchema(s, &resolvers.RootResolver{}, graphql.UseFieldResolvers())

	gql.Post("/", func(c *fiber.Ctx) error {
		req := &GQLRequest{}
		err := c.BodyParser(req)
		if err != nil {
			log.Errorf("gql req, err=%v", err)
			return c.Status(400).JSON(fiber.Map{
				"status":  400,
				"message": "Invalid GraphQL Request. (" + err.Error() + ")",
			})
		}

		if err != nil {
			log.Errorf("session, err=%v", err)
			return c.Status(500).JSON(fiber.Map{
				"status":  500,
				"message": "Failed to get session from store.",
			})
		}

		rCtx := context.WithValue(context.Background(), utils.RequestCtxKey, c)
		rCtx = context.WithValue(rCtx, utils.UserKey, c.Locals("user"))
		result := schema.Exec(rCtx, req.Query, req.OperationName, req.Variables)

		status := 200

		if len(result.Errors) > 0 {
			status = 400
		}

		return c.Status(status).JSON(result)
	})

	return gql
}