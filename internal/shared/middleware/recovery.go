package middleware

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/labstack/echo/v4"
)

// Recovery catches panics in handlers, logs the stack trace, and returns 500.
// Uses a growing buffer to capture deep stack traces that exceed 4KB.
func Recovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 4096)
					const maxStackBuf = 64 * 1024
					for {
						n := runtime.Stack(buf, false)
						if n < len(buf) {
							buf = buf[:n]
							break
						}
						if len(buf) >= maxStackBuf {
							buf = buf[:n]
							break
						}
						buf = make([]byte, len(buf)*2)
					}
					// Truncate panic value to prevent PII/secrets from leaking into logs.
					panicStr := fmt.Sprintf("%v", r)
					if len(panicStr) > 200 {
						panicStr = panicStr[:200] + "...[truncated]"
					}
					slog.Error("panic recovered",
						"error", panicStr,
						"stack", string(buf),
						"path", c.Request().URL.Path,
					)
					if !c.Response().Committed {
						c.Error(echo.ErrInternalServerError)
					}
				}
			}()
			return next(c)
		}
	}
}
