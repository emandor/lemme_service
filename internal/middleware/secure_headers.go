package middleware

import (
	"net/url"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/helmet/v2"
)

func SecureHeadersStrict() fiber.Handler {
	clientURL := os.Getenv("CLIENT_URL")
	env := os.Getenv("APP_ENV")

	u, err := url.Parse(clientURL)
	if err != nil {
		panic("invalid CLIENT_URL")
	}

	// protocol for ws
	wsScheme := "ws:"
	if u.Scheme == "https" {
		wsScheme = "wss:"
	}

	// dynamic CSP
	csp := "default-src 'self'; " +
		"img-src 'self' " + clientURL + " data:; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"font-src 'self'; " +
		"connect-src 'self' " + wsScheme + " https:; " +
		"frame-ancestors 'none';"

	// permissive for dev
	if env == "dev" {
		return helmet.New(helmet.Config{
			ContentSecurityPolicy:     csp,
			CrossOriginEmbedderPolicy: "unsafe-none",
			CrossOriginOpenerPolicy:   "same-origin",
			CrossOriginResourcePolicy: "cross-origin",
		})
	}

	// production strict
	return helmet.New(helmet.Config{
		ContentSecurityPolicy:     csp,
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	})
}
