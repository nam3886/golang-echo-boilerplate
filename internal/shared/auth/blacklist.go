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
