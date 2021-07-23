package emotes

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
)

const chunkSize = 1024 * 1024

var (
	errInternalServer = []byte(`{"status":500,"message":"internal server error"}`)
	errInvalidRequest = `{"status":400,"message":"%s"}`
	errAccessDenied   = `{"status":403,"message":"%s"}`
)

func GetEmoteRoute(router fiber.Router) {
	// Get Emote
	router.Get("/:emote", middleware.RateLimitMiddleware("get-emote", 30, 6*time.Second),
		func(c *fiber.Ctx) error {
			// Parse Emote ID
			id, err := primitive.ObjectIDFromHex(c.Params("emote"))
			if err != nil {
				return restutil.MalformedObjectId().Send(c)
			}

			// Fetch emote data
			var emote datastructure.Emote
			if err := cache.FindOne(c.Context(), "emotes", "", bson.M{
				"_id": id,
			}, &emote); err != nil {
				if err == mongo.ErrNoDocuments {
					return restutil.ErrUnknownEmote().Send(c)
				}
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			// Fetch emote owner
			var owner *datastructure.User
			if err := cache.FindOne(c.Context(), "users", "", bson.M{
				"_id": emote.OwnerID,
			}, &owner); err != nil {
				if err != mongo.ErrNoDocuments {
					return restutil.ErrInternalServer().Send(c, err.Error())
				}
			}

			response := restutil.CreateEmoteResponse(&emote, owner)

			b, err := json.Marshal(&response)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			return c.Send(b)
		})

	// OEmbed
	router.Get("/oembed/:emote.json", func(c *fiber.Ctx) error {
		emoteID := c.Params("emote") // Get the emote ID parameter
		pageTitle := c.Query("page-title", "7TV")

		// Get the emote's data from DB
		var emote *datastructure.Emote
		var owner *datastructure.User
		if id, err := primitive.ObjectIDFromHex(emoteID); err == nil {
			if err := cache.FindOne(c.Context(), "emotes", "", bson.M{
				"_id": id,
			}, &emote); err != nil {
				return c.Status(400).Send([]byte("Unknown Emote: " + err.Error()))
			}
			if err := cache.FindOne(c.Context(), "users", "", bson.M{
				"_id": emote.OwnerID,
			}, &owner); err != nil {
				owner = &datastructure.User{}
			}

			c.Set("Content-Type", "application/json")

			b, err := json.Marshal(OEmbedData{
				Title:        pageTitle,
				AuthorName:   owner.DisplayName,
				AuthorURL:    utils.GetEmotePageURL(emote.ID.Hex()),
				ProviderName: "7TV.APP - It's like a third party thing",
				ProviderURL:  configure.Config.GetString("website_url"),
			})
			if err != nil {
				return c.Status(400).Send([]byte(err.Error()))
			}

			return c.Status(200).Send(b)
		} else {
			return c.Status(400).Send([]byte(err.Error()))
		}
	})
}

type OEmbedData struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
}
