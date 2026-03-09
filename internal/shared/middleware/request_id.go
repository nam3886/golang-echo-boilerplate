package middleware

import (
	"context"

	"github.com/gnha/gnha-services/internal/shared/netutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// isValidRequestID checks that the ID contains only safe characters
// (alphanumeric, hyphen, underscore, dot) and is within length bounds.
// Prevents HTTP response splitting and log injection via the X-Request-ID header.
func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		isSafe := r == '-' || r == '_' || r == '.'
		if !isAlpha && !isDigit && !isSafe {
			return false
		}
	}
	return true
}

// RequestID generates a UUID request ID if X-Request-ID header is missing or invalid,
// and injects it into both the response header and request context.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get("X-Request-ID")
			if !isValidRequestID(id) {
				id = uuid.NewString()
			}
			c.Response().Header().Set("X-Request-ID", id)

			ctx := context.WithValue(c.Request().Context(), requestIDKey, id)
			ctx = netutil.WithClientIP(ctx, c.RealIP())
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
