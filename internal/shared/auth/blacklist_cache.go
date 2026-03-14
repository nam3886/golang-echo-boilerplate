package auth

import (
	"sync"
	"time"
)

// BlacklistCache is a simple in-memory TTL cache for JTI blacklist entries.
// Used as fallback when Redis is unreachable and BLACKLIST_FAIL_OPEN=true.
// On Redis recovery, the cache is populated again on subsequent successful checks,
// so the window of divergence is bounded by BlacklistCacheTTL.
type BlacklistCache struct {
	mu      sync.RWMutex
	entries map[string]time.Time // jti → token expiry time
	ttl     time.Duration
}

// NewBlacklistCache creates a new in-memory blacklist cache with the given TTL.
func NewBlacklistCache(ttl time.Duration) *BlacklistCache {
	return &BlacklistCache{entries: make(map[string]time.Time), ttl: ttl}
}

// Set stores a JTI with its token expiry time.
func (c *BlacklistCache) Set(jti string, tokenExpiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[jti] = tokenExpiry
}

// Contains returns true if jti is in the cache and not yet expired.
func (c *BlacklistCache) Contains(jti string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	exp, ok := c.entries[jti]
	if !ok {
		return false
	}
	return time.Now().Before(exp)
}

// Evict removes expired entries. Should be called periodically.
func (c *BlacklistCache) Evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for jti, exp := range c.entries {
		if now.After(exp) {
			delete(c.entries, jti)
		}
	}
}
