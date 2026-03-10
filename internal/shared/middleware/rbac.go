package middleware

import (
	"github.com/gnha/gnha-services/internal/shared/auth"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/labstack/echo/v4"
)

// Permission represents an RBAC permission string.
type Permission string

const (
	PermUserRead   Permission = "user:read"
	PermUserWrite  Permission = "user:write"
	PermUserDelete Permission = "user:delete"
	PermAdminAll   Permission = "admin:*"
	// ADD_PERMISSION_HERE — scaffold injects new permissions above this line.
)

// RequirePermission checks that the authenticated user has all required permissions.
func RequirePermission(perms ...Permission) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := auth.UserFromContext(c.Request().Context())
			if user == nil {
				return sharederr.ErrUnauthorized()
			}
			for _, p := range perms {
				if !user.HasPermission(string(p)) {
					return sharederr.ErrForbidden()
				}
			}
			return next(c)
		}
	}
}

// RequireRole checks that the authenticated user has one of the allowed roles.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := auth.UserFromContext(c.Request().Context())
			if user == nil {
				return sharederr.ErrUnauthorized()
			}
			for _, r := range roles {
				if user.Role == r {
					return next(c)
				}
			}
			return sharederr.ErrForbidden()
		}
	}
}
