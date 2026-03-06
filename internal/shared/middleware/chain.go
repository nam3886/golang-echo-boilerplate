package middleware

import (
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// SetupMiddleware configures the full middleware chain in correct order.
func SetupMiddleware(e *echo.Echo, cfg *config.Config, rdb *redis.Client) {
	// 1. OTel HTTP tracing (wraps handler to create spans per request)
	e.Use(echo.WrapMiddleware(otelhttp.NewMiddleware(cfg.AppName)))
	// 2. Recovery
	e.Use(Recovery())
	// 3. Request ID
	e.Use(RequestID())
	// 4. Request Logger (with sanitization)
	e.Use(RequestLogger())
	// 5. Body Limit
	e.Use(echomw.BodyLimit("10M"))
	// 6. Gzip
	e.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5}))
	// 7. Security Headers
	e.Use(SecurityHeaders())
	// 8. CORS
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
	// 9. Global Timeout (30s default)
	e.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))
	// 10. Rate Limiting (100 req/min)
	e.Use(RateLimit(rdb, 100, time.Minute))

	// 11. Centralized error handler
	e.HTTPErrorHandler = ErrorHandler

	// NOTE: Auth + RBAC applied at route group level, not global
}
