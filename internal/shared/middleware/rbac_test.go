package middleware

import (
	"net/http"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/labstack/echo/v4"
)

// injectClaims returns middleware that puts a fake authenticated user into context.
func injectClaims(userID, role string, perms []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := &auth.TokenClaims{}
			claims.UserID = userID
			claims.Role = role
			claims.Permissions = perms
			ctx := auth.WithUser(c.Request().Context(), claims)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func okHandler(c echo.Context) error { return c.String(http.StatusOK, "ok") }
