package middleware

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/labstack/echo/v4"
)

// Recovery catches panics in handlers, logs the stack trace, and returns 500.
func Recovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					slog.Error("panic recovered",
						"error", fmt.Sprintf("%v", r),
						"stack", string(buf[:n]),
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
