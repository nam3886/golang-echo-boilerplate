package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Blacklister is the interface for blacklisting JWT tokens by JTI.
// Implemented by RedisBlacklister; can be stubbed in tests.
type Blacklister interface {
	Blacklist(ctx context.Context, jti string, expiry time.Time) error
}

// RedisBlacklister implements Blacklister using Redis.
type RedisBlacklister struct {
	rdb *redis.Client
}

// NewRedisBlacklister creates a RedisBlacklister.
// Panics if rdb is nil.
func NewRedisBlacklister(rdb *redis.Client) *RedisBlacklister {
	if rdb == nil {
		panic("NewRedisBlacklister: rdb must not be nil")
	}
	return &RedisBlacklister{rdb: rdb}
}

// Blacklist adds jti to the Redis blacklist with TTL until expiry.
func (b *RedisBlacklister) Blacklist(ctx context.Context, jti string, expiry time.Time) error {
	return BlacklistToken(ctx, b.rdb, jti, expiry)
}
