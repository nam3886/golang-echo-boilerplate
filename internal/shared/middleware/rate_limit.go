package middleware

// Redis Failure Mode Policy:
//
// Rate limiter: FAIL OPEN — allows request through on Redis error.
// Rationale: rate limiting is best-effort; a Redis outage must not take down the service.
//
// Blacklist (auth.go): FAIL CLOSED — rejects request on Redis error.
// Rationale: an unverified token must not be accepted. Security trumps availability.

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

// slidingWindowLua atomically removes expired entries, adds the current request,
// counts the window members, and refreshes the TTL — all in a single round-trip.
var slidingWindowLua = redis.NewScript(`
local key = KEYS[1]
local window_start = ARGV[1]
local score = ARGV[2]
local member = ARGV[3]
local ttl = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, '0', window_start)
redis.call('ZADD', key, score, member)
local count = redis.call('ZCARD', key)
redis.call('EXPIRE', key, ttl)
return count
`)

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

	// Member combines millisecond timestamp with a random suffix to prevent
	// collision when two requests arrive within the same millisecond tick.
	member := strconv.FormatInt(now.UnixNano(), 10) + ":" + strconv.FormatUint(uint64(rand.Uint32()), 16) //nolint:gosec // collision avoidance, not security

	count, err := slidingWindowLua.Run(ctx, rdb, []string{key},
		fmt.Sprintf("%d", windowStart.UnixMilli()),
		float64(now.UnixMilli()),
		member,
		int(window.Seconds()),
	).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}
