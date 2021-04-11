package server

import (
	"net"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/jwt"
	apiv2 "github.com/SevenTV/ServerGo/server/api/v2"
	"github.com/SevenTV/ServerGo/server/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/SevenTV/ServerGo/configure"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Server struct {
	app      *fiber.App
	listener net.Listener
}

type CustomLogger struct{}

func (*CustomLogger) Write(data []byte) (n int, err error) {
	log.Infoln(utils.B2S(data))
	return len(data), nil
}

func New() *Server {
	l, err := net.Listen(configure.Config.GetString("conn_type"), configure.Config.GetString("conn_uri"))
	if err != nil {
		log.Fatalf("failed to start listner for http server, err=%v", err)
		return nil
	}

	server := &Server{
		app: fiber.New(fiber.Config{
			BodyLimit:                    2e16,
			StreamRequestBody:            true,
			DisablePreParseMultipartForm: true,
		}),
		listener: l,
	}

	server.app.Use(cors.New(cors.Config{
		AllowOrigins:  "*",
		ExposeHeaders: "X-Collection-Size",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE",
	}))

	server.app.Use(logger.New(logger.Config{
		Output: &CustomLogger{},
	}))

	server.app.Use(recover.New())

	server.app.Use(func(c *fiber.Ctx) error {
		nodeID := configure.Config.GetString("node_id")
		if nodeID != "" {
			c.Set("X-Node-ID", nodeID)
		}
		delete := true
		auth := c.Cookies("auth")
		if auth != "" {
			splits := strings.Split(auth, ".")
			if len(splits) != 3 {
				pl := &middleware.PayloadJWT{}
				if err := jwt.Verify(splits, pl); err == nil {
					if pl.CreatedAt.After(time.Now().Add(-time.Hour * 24 * 60)) {
						delete = false
						c.Cookie(&fiber.Cookie{
							Name:     "auth",
							Value:    auth,
							Domain:   configure.Config.GetString("cookie_domain"),
							Expires:  time.Now().Add(time.Hour * 24 * 14),
							Secure:   configure.Config.GetBool("cookie_secure"),
							HTTPOnly: false,
						})
					}
				}
			}
			if delete {
				c.Cookie(&fiber.Cookie{
					Name:     "auth",
					Domain:   configure.Config.GetString("cookie_domain"),
					MaxAge:   -1,
					Secure:   configure.Config.GetBool("cookie_secure"),
					HTTPOnly: false,
				})
			}
		}

		return c.Next()
	})

	apiv2.API(server.app)

	server.app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("OK")
	})

	server.app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(&fiber.Map{
			"status":  404,
			"message": "We don't know what you're looking for.",
		})
	})

	go func() {
		err = server.app.Listener(server.listener)
		if err != nil {
			log.Errorf("failed to start http server, err=%v", err)
		}
	}()

	return server
}

func (s *Server) Shutdown() error {
	return s.listener.Close()
}
