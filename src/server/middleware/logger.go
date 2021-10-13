package middleware

import (
	"time"

	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

func Logger() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		var (
			err interface{}
		)
		func() {
			defer func() {
				err = recover()
			}()
			err = c.Next()
		}()
		if err != nil {
			_ = c.SendStatus(500)
		}

		l := logrus.WithFields(logrus.Fields{
			"status":   c.Response().StatusCode(),
			"path":     utils.B2S(c.Request().RequestURI()),
			"duration": time.Since(start) / time.Millisecond,
			"ip":       c.Get("Cf-Connecting-IP"),
		})

		if err != nil {
			l = l.WithField("error", err)
		}
		l.Info("logger")
		return nil
	}
}
