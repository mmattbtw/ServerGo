package gql

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	mutation_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/mutation"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gobuffalo/packr/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/graph-gophers/graphql-go"

	log "github.com/sirupsen/logrus"
)

type GQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operation_name"`
}

var Ctx = context.Background()

type RootResolver struct {
	*query_resolvers.QueryResolver
	*mutation_resolvers.MutationResolver
}

func GQL(app fiber.Router) fiber.Router {
	gql := app.Group("/gql", middleware.UserAuthMiddleware(false))

	box := packr.New("gql", "./scheme")

	s, err := box.FindString("scheme.gql")

	if err != nil {
		panic(err)
	}

	schema := graphql.MustParseSchema(s, &RootResolver{
		&query_resolvers.QueryResolver{},
		&mutation_resolvers.MutationResolver{},
	}, graphql.UseFieldResolvers())

	rl := configure.Config.GetIntSlice("limits.route.gql")
	gql.Use(cors.New(cors.Config{
		AllowOrigins:  fmt.Sprintf("%v,%v,%v", configure.Config.GetString("website_url"), "chrome-extension://*", "moz-extension://*"),
		ExposeHeaders: "X-Collection-Size",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE",
	}))
	gql.Use(middleware.RateLimitMiddleware("gql", int32(rl[0]), time.Millisecond*time.Duration(rl[1])))
	gql.Post("/", func(c *fiber.Ctx) error {
		req := &GQLRequest{}
		err := c.BodyParser(req)
		if err != nil {
			return err
		}
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

		rCtx := context.WithValue(Ctx, utils.RequestCtxKey, c)
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
