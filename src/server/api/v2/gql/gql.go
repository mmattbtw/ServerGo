package gql

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
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
		req := &GQLRequest{}
		err := c.BodyParser(req)
		if err != nil {
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

		start := time.Now()

		rCtx := context.WithValue(Ctx, utils.RequestCtxKey, c)
		rCtx = context.WithValue(rCtx, utils.UserKey, c.Locals("user"))
		result := schema.Exec(rCtx, req.Query, req.OperationName, req.Variables)

		go func() {
			sniffer := configure.Config.GetString("gql_sniffer")
			if sniffer == "" {
				return
			}
			data, err := json.Marshal(Query{
				ID:         primitive.NewObjectIDFromTimestamp(start),
				IP:         c.Get("Cf-Connecting-IP"),
				Query:      req.Query,
				Origin:     c.Get("Origin"),
				UserAgent:  c.Get("User-Agent"),
				Variables:  req.Variables,
				TimeTaken:  time.Since(start),
				RawHeaders: utils.B2S(c.Request().Header.RawHeaders()),
			})
			if err != nil {
				return
			}
			resp, err := http.Post(sniffer, "application/json", bytes.NewBuffer(data))
			if err != nil {
				return
			}
			_ = resp.Body.Close()
		}()

		status := 200

		if len(result.Errors) > 0 {
			status = 400
		}

		return c.Status(status).JSON(result)
	})

	return gql
}
