package middleware

import (
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

type AuditedRoute func(*fiber.Ctx) (int, []byte, *datastructure.AuditLog)

func AuditRoute(r AuditedRoute) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		statusCode, body, auditEntry := r(c)

		if auditEntry != nil {
			_, err := mongo.Database.Collection("audit").InsertOne(mongo.Ctx, auditEntry)
			if err != nil {
				log.Errorf("audit, err=%v", err)
			}
		}

		return c.Status(statusCode).Send(body)
	}
}
