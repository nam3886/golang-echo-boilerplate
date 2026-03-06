package middleware

import (
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
)

// SetupMiddleware configures the full middleware chain in correct order.
func SetupMiddleware(e *echo.Echo, cfg *config.Config, rdb *redis.Client) {
	// 1. Recovery
	e.Use(Recovery())
	// 2. Request ID
	e.Use(RequestID())
	// 3. Request Logger (with sanitization)
	e.Use(RequestLogger())
	// 4. Body Limit
	e.Use(echomw.BodyLimit("10M"))
	// 5. Gzip
	e.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5}))
	// 6. Security Headers
	e.Use(SecurityHeaders())
	// 7. CORS
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: cfg.CORSOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Accept", "Authorization", "Content-Type",
			"X-Request-ID", "Connect-Protocol-Version",
		},
		AllowCredentials: true,
		MaxAge:           3600,
	}))
	// 8. Global Timeout (30s default)
	e.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))
	// 9. Rate Limiting (100 req/min)
	e.Use(RateLimit(rdb, 100, time.Minute))

	// 10. Centralized error handler
	e.HTTPErrorHandler = ErrorHandler

	// NOTE: Auth + RBAC applied at route group level, not global
}
