package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RateLimit applies a Redis sliding window rate limiter.
// On Redis failure the limiter fails open (allows the request) to prevent
// a Redis outage from taking down the entire service.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := rateLimitKey(c)
			count, err := slidingWindowCount(c.Request().Context(), rdb, key, window)
			if err != nil {
				// Fail open: rate limiting is a best-effort control.
				// A Redis outage must not bring down the service.
				slog.ErrorContext(c.Request().Context(), "rate limiter redis error", "err", err)
				return next(c)
			}
			if count > int64(limit) {
				c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				return echo.NewHTTPError(429, "rate limit exceeded")
			}
			return next(c)
		}
	}
}

func rateLimitKey(c echo.Context) string {
	// Rate limiting is IP-based. User-keying is not possible because
	// this middleware runs before Auth in the global chain.
	return "ratelimit:ip:" + c.RealIP()
}

func slidingWindowCount(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	now := time.Now()
	windowStart := now.Add(-window)

	// Member combines nanosecond timestamp with a random suffix to prevent
	// collision when two requests arrive within the same nanosecond tick.
	member := strconv.FormatInt(now.UnixNano(), 10) + ":" + strconv.FormatUint(uint64(rand.Uint32()), 16) //nolint:gosec // collision avoidance, not security

	// Score: milliseconds for window range queries.
	// Member: nanoseconds for uniqueness (multiple requests per ms).
	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: member})
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return countCmd.Val(), nil
}
