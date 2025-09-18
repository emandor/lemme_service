package middleware

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type SessionProvider interface {
	Rdb() *redis.Client
	CookieName() string
}

func AuthSession(reg SessionProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sid := c.Cookies(reg.CookieName())
		if sid == "" {
			return c.Status(401).SendString("unauthorized")
		}
		val, err := reg.Rdb().Get(context.Background(), "sess:"+sid).Result()
		if err != nil {
			return c.Status(401).SendString("unauthorized")
		}
		uid, _ := strconv.ParseInt(val, 10, 64)
		c.Locals("userID", uid)
		return c.Next()
	}
}
