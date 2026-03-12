package middleware

import (
	"log/slog"
	"strings"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Auth validates JWT Bearer tokens and injects the user into context.
func Auth(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractBearerToken(c)
			if token == "" {
				return sharederr.ErrUnauthorized()
			}

			claims, err := auth.ValidateAccessToken(cfg, token)
			if err != nil {
				return sharederr.ErrUnauthorized()
			}

			// Blacklist (logout) check — FAIL CLOSED: any Redis error rejects the token.
			// Rationale: an unverified token must not be accepted. Security trumps availability.
			// See rate_limit.go for the contrasting fail-open policy used there.
			ctx := c.Request().Context()
			blacklisted, err := auth.IsBlacklisted(ctx, rdb, claims.ID)
			if err != nil {
				slog.ErrorContext(ctx, "blacklist check failed", "err", err, "jti", claims.ID)
				return sharederr.ErrUnauthorized()
			}
			if blacklisted {
				return sharederr.ErrUnauthorized()
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
