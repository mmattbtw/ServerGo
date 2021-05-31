package chatterino

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"
)

func Chatterino(app fiber.Router) fiber.Router {
	chatterino := app.Group("/chatterino")

	chatterino.Get("/version/:platform/:branch", func(c *fiber.Ctx) error {
		portableDownload := configure.Config.GetString(fmt.Sprintf("chatterino.portable_download.%v", c.Params("platform")))
		download := configure.Config.GetString(fmt.Sprintf(fmt.Sprintf("chatterino.download.%v", c.Params("platform"))))
		update := configure.Config.GetString(fmt.Sprintf("chatterino.update.%v", c.Params("platform")))
		version := configure.Config.GetString("chatterino.version")

		result := VersionResult{
			portableDownload,
			download,
			update,
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
	Download         string `json:"download"`
	UpdateEXE        string `json:"updateexe"`
	Version          string `json:"version"`
}
