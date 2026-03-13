package middleware

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// SetupMiddleware configures the full middleware chain in correct order.
//
// Middleware layers (three tiers):
//  1. Echo global (this function) — runs on every request
//  2. Echo group (in routes.go) — Auth per service
//  3. Connect interceptor (in routes.go) — RBACInterceptor + protovalidate per procedure
func SetupMiddleware(e *echo.Echo, cfg *config.Config, rdb *redis.Client) {
	// IMPORTANT: Configure trusted proxy for accurate client IP (rate limiting, audit).
	// Behind a reverse proxy, set: e.IPExtractor = echo.ExtractIPFromXFFHeader()
	// Without this, X-Forwarded-For can be spoofed to bypass rate limiting.

	// Warn if production uses default IP extraction (spoofable via X-Forwarded-For).
	if cfg.IsProduction() && e.IPExtractor == nil {
		slog.Error("rate limiter uses default IPExtractor in production; " +
			"configure e.IPExtractor = echo.ExtractIPFromXFFHeader() for accurate client IP behind reverse proxy")
	}

	// 0. HTTPS redirect — production only. Use e.Pre() so the redirect fires before
	// routing. Not enabled in dev/staging to avoid breaking local HTTP connections.
	if cfg.IsProduction() {
		e.Pre(echomw.HTTPSRedirect())
	}

	// 1. OTel HTTP tracing (wraps handler to create spans per request)
	e.Use(echo.WrapMiddleware(otelhttp.NewMiddleware(cfg.AppName)))
	// 2. Recovery
	e.Use(Recovery())
	// 3. Request ID
	e.Use(RequestID())
	// 4. Rate Limiting — before body parsing to prevent resource-exhaustion DDoS.
	// Configurable via RATE_LIMIT_RPM and RATE_LIMIT_WINDOW env vars (default: 100 req/min).
	e.Use(RateLimit(rdb, cfg.RateLimitRPM, cfg.RateLimitWindow))
	// 5. Request Logger (with sanitization)
	e.Use(RequestLogger())
	// 6. Body Limit
	e.Use(echomw.BodyLimit("10M"))
	// 7. Gzip
	e.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5}))
	// 8. Security Headers
	e.Use(SecurityHeaders(cfg))
	// 9. CORS
	// Warn if CORS allows localhost in production — likely misconfiguration.
	if cfg.IsProduction() {
		for _, o := range cfg.CORSOrigins {
			if strings.Contains(o, "localhost") || strings.Contains(o, "127.0.0.1") {
				slog.Warn("CORS_ORIGINS contains localhost in production — likely misconfiguration", "origins", cfg.CORSOrigins)
				break
			}
		}
	}
	// Only enable credentials when origins are explicitly listed (not wildcard).
	// Access-Control-Allow-Origin: * with AllowCredentials: true is rejected by browsers.
	allowCreds := !slices.Contains(cfg.CORSOrigins, "*")
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: cfg.CORSOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Accept", "Authorization", "Content-Type",
			"X-Request-ID", "Connect-Protocol-Version",
		},
		AllowCredentials: allowCreds,
		MaxAge:           3600,
	}))
	// 10. Global Timeout (configurable, default 30s)
	// WARNING: ContextTimeout cancels the request context after the configured duration.
	// If a handler writes to the DB and then publishes an event, the context may cancel
	// between the two operations. Handlers doing multi-step writes should use their own
	// deadline or check ctx.Err() between steps.
	e.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
		Timeout: cfg.RequestTimeout,
	}))

	// 11. Centralized error handler
	e.HTTPErrorHandler = ErrorHandler

	// NOTE: Auth + RBAC applied at route group level, not global
}
