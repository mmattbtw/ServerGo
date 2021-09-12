package rest

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/cosmetics"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/emotes"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/users"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/rewrite/v2"
)

func RestV2(app fiber.Router) fiber.Router {
	restGroup := app.Group("/")
	restGroup.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")

		return c.Next()
	})
	restGroup.Use(rewrite.New(rewrite.Config{
		Rules: map[string]string{
			"/v2/badges": "/v2/cosmetics",
		},
	}))

	emoteGroup := restGroup.Group("/emotes")
	emotes.CreateEmoteRoute(emoteGroup)
	emotes.GetGlobalEmotes(emoteGroup)
	emotes.GetEmoteRoute(emoteGroup)

	userGroup := restGroup.Group("/users")
	users.GetUser(userGroup)
	users.GetChannelEmotesRoute(userGroup)

	cosmeticsGroup := restGroup.Group("/cosmetics")
	cosmetics.GetBadges(cosmeticsGroup)

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

	restGroup.Get("/platforms/:id/:variant?", func(c *fiber.Ctx) error {
		ctx := c.Context()

		platformID := c.Params("id")
		variantID := c.Params("variant")
		key := fmt.Sprintf("hits:platform:%v", platformID)
		if variantID != "" {
			key = key + fmt.Sprintf(":%v", variantID)
		}

		// Find platform
		var platforms []*Platform
		if err := configure.Config.UnmarshalKey("platforms", &platforms); err != nil {
			return c.Status(500).SendString("Error decoding the config")
		}

		// Find the requested platform
		var platform *Platform
		var variant *PlatformVariant
		for _, p := range platforms {
			if p.ID != platformID {
				continue
			}

			platform = p
			if p.Variants != nil {
				for _, v := range *p.Variants {
					if v.ID != variantID {
						continue
					}

					variant = &v
					break
				}
			}
			break
		}
		if platform == nil {
			return c.Status(400).SendString("Unknown Platform")
		}

		redis.Client.Incr(ctx, key)
		if variant != nil {
			return c.Redirect(variant.URL)
		} else if platform != nil && platform.URL != "" {
			return c.Redirect(platform.URL, 301)
		}

		return c.Redirect(configure.Config.GetString("website_url"))
	})

	return nil
}

type PlatformsResult struct {
	Platforms []*Platform `json:"platforms"`
}

type Platform struct {
	ID         string             `mapstructure:"id" json:"id"`
	VersionTag string             `mapstructure:"version_tag" json:"version_tag"`
	New        bool               `mapstructure:"new" json:"new"`
	URL        string             `mapstructure:"url" json:"url"`
	Variants   *[]PlatformVariant `mapstructure:"variants" json:"variants"`
}

type PlatformVariant struct {
	Name        string `json:"name" mapstructure:"name"`
	ID          string `json:"id" mapstructure:"id"`
	Author      string `json:"author" mapstructure:"author"`
	Version     string `json:"version" mapstructure:"version"`
	Description string `json:"description" mapstructure:"description"`
	URL         string `json:"url" mapstructure:"url"`
}
