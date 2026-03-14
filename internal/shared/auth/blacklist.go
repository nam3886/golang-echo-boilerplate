// Package auth provides JWT token management including blacklisting via Redis.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// blacklistPrefix must match the key prefix checked in internal/shared/middleware/auth.go.
const blacklistPrefix = "blacklist:"

// BlacklistToken adds a JWT token ID (jti) to the Redis blacklist.
// The key expires when the token would have expired, preventing unbounded Redis growth.
// Returns nil immediately if the token is already expired.
func BlacklistToken(ctx context.Context, rdb *redis.Client, jti string, tokenExpiry time.Time) error {
	ttl := time.Until(tokenExpiry)
	if ttl <= 0 {
		return nil // token already expired, no need to blacklist
	}
	key := blacklistPrefix + jti
	if err := rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("blacklisting token %s: %w", jti, err)
	}
	return nil
}

// IsBlacklisted checks if a JWT token ID is in the Redis blacklist.
func IsBlacklisted(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	key := blacklistPrefix + jti
	n, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("checking blacklist for %s: %w", jti, err)
	}
	return n > 0, nil
}

// IsBlacklistedWithCache checks the Redis blacklist and maintains a local in-memory cache.
// On successful Redis hit: populates cache so it survives brief Redis outages.
// On Redis error with fail-open: consults cache — if jti is cached as blacklisted, still denies.
// tokenExpiry is required to set correct TTL in the local cache.
// Returns (blacklisted bool, redisErr error).
func IsBlacklistedWithCache(
	ctx context.Context,
	rdb *redis.Client,
	cache *BlacklistCache,
	jti string,
	tokenExpiry time.Time,
) (blacklisted bool, err error) {
	key := blacklistPrefix + jti
	n, redisErr := rdb.Exists(ctx, key).Result()
	if redisErr != nil {
		// Redis unavailable — fall back to local cache.
		// If jti was previously seen as blacklisted, honour that (deny).
		// If not in cache, caller decides based on fail-open/fail-closed policy.
		return cache.Contains(jti), fmt.Errorf("checking blacklist for %s: %w", jti, redisErr)
	}

	isBlacklisted := n > 0
	if isBlacklisted {
		// Populate cache so Redis-outage fallback can still deny this token.
		cache.Set(jti, tokenExpiry)
	}
	return isBlacklisted, nil
}
