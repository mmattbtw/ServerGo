package rest

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/badges"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/emotes"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/users"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func RestV2(app fiber.Router) fiber.Router {
	restGroup := app.Group("/")
	restGroup.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")

		return c.Next()
	})
	restGroup.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE",
	}))

	emoteGroup := restGroup.Group("/emotes")
	emotes.CreateEmoteRoute(emoteGroup)
	emotes.GetGlobalEmotes(emoteGroup)
	emotes.GetEmoteRoute(emoteGroup)

	userGroup := restGroup.Group("/users")
	users.GetUser(userGroup)
	users.GetChannelEmotesRoute(userGroup)

	badgeGroup := restGroup.Group("/badges")
	badges.GetBadges(badgeGroup)

	restGroup.Get("/webext", func(c *fiber.Ctx) error {
		// result := &WebExtResult{}

		var platforms []*Platform
		err := configure.Config.UnmarshalKey("platforms", &platforms)
		if err != nil {
			return c.Status(500).SendString("Error decoding the config")
		}

		j, err := json.Marshal(platforms)
		if err != nil {
			return c.Status(500).SendString(fmt.Sprintf("Error decoding the config: %v", err.Error()))
		}

		return c.Send(j)
	})

	return nil
}

type PlatformsResult struct {
	Platforms []*Platform `json:"platforms"`
}

type Platform struct {
	ID          string             `mapstructure:"id" json:"id"`
	VersionTag  string             `mapstructure:"version_tag" json:"version_tag"`
	New         bool               `mapstructure:"new" json:"new"`
	Subversions *[]PlatformVariant `mapstructure:"variants" json:"variants"`
}

type PlatformVariant struct {
	Name    string `json:"name" mapstructure:"name"`
	Author  string `json:"author" mapstructure:"author"`
	Version string `json:"version" mapstructure:"version"`
	URL     string `json:"url" mapstructure:"url"`
}
