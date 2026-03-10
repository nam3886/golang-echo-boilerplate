package middleware

import (
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/labstack/echo/v4"
)

// SecurityHeaders adds standard security headers to responses.
// HSTS is only set outside development to avoid browser HTTPS-forcing on localhost.
func SecurityHeaders(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "0")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("Content-Security-Policy", "default-src 'self'")
			if cfg.AppEnv != "development" {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			return next(c)
		}
	}
}
