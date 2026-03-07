package middleware

import (
	"strings"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/config"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Auth validates JWT Bearer tokens and injects the user into context.
func Auth(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractBearerToken(c)
			if token == "" {
				return domainerr.ErrUnauthorized()
			}

			claims, err := auth.ValidateAccessToken(cfg, token)
			if err != nil {
				return domainerr.ErrUnauthorized()
			}

			// Check token blacklist (logout). Fail closed: any Redis error rejects the token.
			ctx := c.Request().Context()
			blacklisted, err := rdb.Exists(ctx, "blacklist:"+claims.RegisteredClaims.ID).Result()
			if err != nil {
				return domainerr.ErrUnauthorized()
			}
			if blacklisted > 0 {
				return domainerr.ErrUnauthorized()
			}

			ctx = auth.WithUser(ctx, claims)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func extractBearerToken(c echo.Context) string {
	header := c.Request().Header.Get("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return header[7:]
	}
	return ""
}
