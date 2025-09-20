package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/helmet/v2"
)

// SecureHeaders -> default security headers
func SecureHeaders() fiber.Handler {
	return helmet.New()
}

// SecureHeadersStrict -> strict mode with CSP & isolation
func SecureHeadersStrict() fiber.Handler {
	return helmet.New(helmet.Config{
		ContentSecurityPolicy: "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " +
			// "img-src 'self' data:; " +
			"font-src 'self'; " +
			"connect-src 'self' wss: https:; " +
			"frame-ancestors 'none';",

		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	})
}
