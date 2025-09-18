package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const ReqIDKey = "reqID"

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid := c.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set("X-Request-ID", rid)
		c.Locals(ReqIDKey, rid)
		return c.Next()
	}
}
