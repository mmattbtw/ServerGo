package badges

import (
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/gofiber/fiber/v2"
)

func GetBadges(router fiber.Router) {
	router.Get("/badges", func(c *fiber.Ctx) error {
		return c.Status(501).SendString(`{"error":"This endpoint is coming soon","status":501}`)
	})
}

type GetBadgesResult struct {
	Badges []*datastructure.Badge `json:"badges"`
}
