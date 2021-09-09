package gql

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	jsoniter "github.com/json-iterator/go"
	"go.mongodb.org/mongo-driver/bson/primitive"

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

type Query struct {
	ID         primitive.ObjectID     `json:"id"`
	IP         string                 `json:"ip"`
	Query      string                 `json:"query"`
	Origin     string                 `json:"origin"`
	UserAgent  string                 `json:"user_agent"`
	TimeTaken  time.Duration          `json:"time_taken"`
	Variables  map[string]interface{} `json:"variables"`
	RawHeaders string                 `json:"raw_headers"`
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func GQL(app fiber.Router) fiber.Router {
	gql := app.Group("/gql", middleware.UserAuthMiddleware(false))

	box := packr.New("gql", "./schema")

	s, err := box.FindString("schema.gql")

	if err != nil {
		log.WithError(err).Fatal("gql failed")
	}

	schema := graphql.MustParseSchema(s, &RootResolver{
		&query_resolvers.QueryResolver{},
		&mutation_resolvers.MutationResolver{},
	}, graphql.UseFieldResolvers())

	rl := configure.Config.GetIntSlice("limits.route.gql")
	origins := configure.Config.GetStringSlice("cors_origins")
	gql.Use(cors.New(cors.Config{
		AllowOrigins: utils.Ternary(configure.Config.GetBool("cors_wildcard"),
			"*",
			fmt.Sprintf("%v,%v,%v,%v", configure.Config.GetString("website_url"), strings.Join(origins, ","), "chrome-extension://*", "moz-extension://*"),
		).(string),
		ExposeHeaders: "X-Collection-Size,X-Created-ID",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE",
	}))
	gql.Use(middleware.RateLimitMiddleware("gql", int32(rl[0]), time.Millisecond*time.Duration(rl[1])))
	gql.Post("/", func(c *fiber.Ctx) error {
		req := GQLRequest{}
		if err := c.BodyParser(&req); err != nil {
			return err
		}
		if err != nil {
			log.WithError(err).Error("gql")
			return c.Status(400).JSON(fiber.Map{
				"status":  400,
				"message": "Invalid GraphQL Request. (" + err.Error() + ")",
			})
		}

		if err != nil {
			log.WithError(err).Error("session")
			return c.Status(500).JSON(fiber.Map{
				"status":  500,
				"message": "Failed to get session from store.",
			})
		}

		extraData := c.Locals("extra_data").(sync.Map)
		extraData.Store("gql_request", req)

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
