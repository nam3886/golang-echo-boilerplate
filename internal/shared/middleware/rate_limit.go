package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RateLimit applies a Redis sliding window rate limiter.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := rateLimitKey(c)
			count, err := slidingWindowCount(c.Request().Context(), rdb, key, window)
			if err != nil {
				// On Redis failure, fail closed (503 Service Unavailable).
				slog.ErrorContext(c.Request().Context(), "rate limiter redis error", "err", err)
				return echo.NewHTTPError(503, "service unavailable")
			}
			if count >= int64(limit) {
				c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				return echo.NewHTTPError(429, "rate limit exceeded")
			}
			return next(c)
		}
	}
}

func rateLimitKey(c echo.Context) string {
	if user := auth.UserFromContext(c.Request().Context()); user != nil {
		return "ratelimit:user:" + user.UserID
	}
	return "ratelimit:ip:" + c.RealIP()
}

func slidingWindowCount(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	now := time.Now()
	windowStart := now.Add(-window)

	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: now.UnixNano()})
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return countCmd.Val(), nil
}
