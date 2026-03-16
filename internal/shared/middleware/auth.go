package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Auth validates JWT Bearer tokens and injects the user into context.
// Blacklist cache is initialised once per middleware instance using cfg.BlacklistCacheTTL
// (default 30s). The cache acts as a fallback when Redis is unreachable and
// BLACKLIST_FAIL_OPEN=true — previously-seen blacklisted JTIs are still denied.
func Auth(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc {
	// Initialise the local blacklist cache once per middleware instance.
	// A 30s default is used when BlacklistCacheTTL is zero (e.g. in unit tests
	// that construct Config directly without going through config.Load).
	cacheTTL := cfg.BlacklistCacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	var (
		once  sync.Once
		cache *auth.BlacklistCache
	)
	initCache := func() *auth.BlacklistCache {
		once.Do(func() {
			cache = auth.NewBlacklistCache(cacheTTL)
			// Periodically evict expired entries to prevent unbounded memory growth.
			go func() {
				ticker := time.NewTicker(5 * time.Minute)
				defer ticker.Stop()
				for range ticker.C {
					cache.Evict()
				}
			}()
		})
		return cache
	}

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

			// Blacklist (logout) check.
			// Default: FAIL CLOSED — any Redis error rejects the token (security over availability).
			// Configurable via BLACKLIST_FAIL_OPEN=true for HA deployments.
			// The local cache provides a safety net during brief Redis outages:
			// previously-seen blacklisted JTIs are still denied even when Redis is down.
			ctx := c.Request().Context()
			tokenExpiry := claims.ExpiresAt.Time
			blacklisted, checkErr := auth.IsBlacklistedWithCache(ctx, rdb, initCache(), claims.ID, tokenExpiry)
			if checkErr != nil {
				slog.ErrorContext(ctx, "blacklist check failed",
					"module", "auth", "operation", "blacklist_check",
					"user_id", claims.UserID, "jti_hash", hashJTI(claims.ID),
					"error_code", "blacklist_unavailable", "retryable", true, "err", checkErr)
				if !cfg.BlacklistFailOpen {
					return sharederr.ErrUnauthorized()
				}
				// fail-open: Redis unreachable — cache answer already returned by IsBlacklistedWithCache.
				// If blacklisted==true the token is denied below; otherwise allow through.
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

// hashJTI returns the first 8 hex characters of the SHA-256 hash of jti.
// Safe to include in logs — identifies the token for correlation without exposing the raw ID.
func hashJTI(jti string) string {
	h := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(h[:4]) // 4 bytes → 8 hex chars
}

func extractBearerToken(c echo.Context) string {
	header := c.Request().Header.Get("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return header[7:]
	}
	return ""
}
