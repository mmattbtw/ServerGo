package emotes

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
)

const chunkSize = 1024 * 1024

var (
	errInternalServer = []byte(`{"status":500,"message":"internal server error"}`)
	errInvalidRequest = `{"status":400,"message":"%s"}`
	errAccessDenied   = `{"status":403,"message":"%s"}`
)

func Emotes(app fiber.Router) fiber.Router {
	emotes := app.Group("/emotes")

	emotes.Get("/oembed/:emote.json", func(c *fiber.Ctx) error {
		emoteID := c.Params("emote") // Get the emote ID parameter
		pageTitle := c.Query("page-title", "7TV")
		fmt.Println("emoteid", emoteID)

		// Get the emote's data from DB
		var emote *mongo.Emote
		var owner *mongo.User
		if id, err := primitive.ObjectIDFromHex(emoteID); err == nil {
			if err := cache.FindOne("emotes", "", bson.M{
				"_id": id,
			}, &emote); err != nil {
				return c.Status(400).Send([]byte("Unknown Emote: " + err.Error()))
			}
			if err := cache.FindOne("users", "", bson.M{
				"_id": emote.OwnerID,
			}, &owner); err != nil {
				owner = &mongo.User{}
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

	CreateRoute(emotes)

	return emotes
}

type OEmbedData struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
}
