package middleware

import (
	"runtime/debug"
	"strings"
	"time"

	"github.com/emandor/lemme_service/internal/config"
	"github.com/emandor/lemme_service/internal/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func RequestLog() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		log := telemetry.L().With().Logger()

		log.Info().Msgf("%s %s %d %v ua=%q ip=%s",
			c.Method(), c.Path(), c.Response().StatusCode(), time.Since(start),
			c.Get("User-Agent"), c.IP(),
		)
		return err
	}
}

func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {

			if r := recover(); r != nil {

				log := telemetry.L().With().Logger()
				log.Error().Msg("panic: recovered")
				log.Error().Msg(string(debug.Stack()))
				_ = c.Status(500).SendString("internal error")
			}
		}()
		return c.Next()
	}
}

func CORS(cfg *config.Config) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSOrigins, ","),
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
