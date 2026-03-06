package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const clientIPKey contextKey = "client_ip"

// RequestID generates a UUID request ID if X-Request-ID header is missing,
// and injects it into both the response header and request context.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get("X-Request-ID")
			if id == "" || len(id) > 128 {
				id = uuid.NewString()
			}
			c.Response().Header().Set("X-Request-ID", id)

			ctx := context.WithValue(c.Request().Context(), requestIDKey, id)
			ctx = context.WithValue(ctx, clientIPKey, c.RealIP())
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// GetRequestID extracts the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetClientIP extracts the client IP from context.
func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(clientIPKey).(string); ok {
		return ip
	}
	return ""
}
