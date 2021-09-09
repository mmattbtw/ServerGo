package middleware

import (
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

func Logger() func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		var (
			err interface{}
		)
		c.Locals("extra_data", sync.Map{})
		func() {
			defer func() {
				err = recover()
			}()
			err = c.Next()
		}()
		if err != nil {
			_ = c.SendStatus(500)
		}

		extraDataSync := c.Locals("extra_data").(sync.Map)
		extraData := map[interface{}]interface{}{}
		extraDataSync.Range(func(key, value interface{}) bool {
			extraData[key] = value
			return true
		})
		l := log.WithFields(log.Fields{
			"status":      c.Response().StatusCode(),
			"path":        utils.B2S(c.Request().RequestURI()),
			"duration":    time.Since(start) / time.Millisecond,
			"ip":          c.Get("Cf-Connecting-IP"),
			"origin":      c.Get("Origin"),
			"user_agent":  c.Get("User-Agent"),
			"extra_data":  extraData,
			"raw_headers": utils.B2S(c.Request().Header.RawHeaders()),
		})

		if err != nil {
			l = l.WithField("error", err)
		}
		l.Info("logger")
		return nil
	}
}
