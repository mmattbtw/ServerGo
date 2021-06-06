package badges

import "github.com/gofiber/fiber/v2"

func GetBadges(router fiber.Router) {
	router.Get("/badges", func(c *fiber.Ctx) error {
		return c.Status(501).SendString(`{"error":"This endpoint is coming soon","status":501}`)
	})
}
