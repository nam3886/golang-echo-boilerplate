package middleware

import (
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// RequestLogger logs method, path, status, latency, trace and user context.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()
			latency := time.Since(start)

			attrs := []any{
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", latency.Milliseconds(),
				"bytes", res.Size,
				"ip", c.RealIP(),
			}

			ctx := req.Context()

			if reqID := GetRequestID(ctx); reqID != "" {
				attrs = append(attrs, "request_id", reqID)
			}

			if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
				attrs = append(attrs, "trace_id", spanCtx.TraceID().String())
			}

			if user := auth.UserFromContext(ctx); user != nil {
				attrs = append(attrs, "user_id", user.UserID)
			}

			if ua := req.UserAgent(); ua != "" {
				attrs = append(attrs, "user_agent", ua)
			}

			if res.Status >= 500 {
				slog.Error("request", attrs...)
			} else if res.Status >= 400 {
				slog.Warn("request", attrs...)
			} else {
				slog.Info("request", attrs...)
			}

			return nil
		}
	}
}

