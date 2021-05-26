package chatterino

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"
)

func Chatterino(app fiber.Router) fiber.Router {
	chatterino := app.Group("/chatterino")

	chatterino.Get("/version/:platform/stable", func(c *fiber.Ctx) error {
		portableDownload := configure.Config.GetString(fmt.Sprintf("chatterino.portable_download.%v", c.Params("platform")))
		version := configure.Config.GetString("chatterino.version")

		result := VersionResult{
			portableDownload,
			version,
		}

		b, err := json.Marshal(result)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.Status(200).Send(b)
	})

	return chatterino
}

type VersionResult struct {
	PortableDownload string `json:"portable_download"`
	Version          string `json:"version"`
}
