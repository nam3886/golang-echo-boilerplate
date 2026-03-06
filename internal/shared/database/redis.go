package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client with retry logic.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}
	opt.PoolSize = 10 * runtime.NumCPU()
	opt.MinIdleConns = 5

	rdb := redis.NewClient(opt)
	ctx := context.Background()

	for i := range 10 {
		if err = rdb.Ping(ctx).Err(); err == nil {
			slog.Info("redis connected", "addr", opt.Addr)
			return rdb, nil
		}
		slog.Warn("redis not ready, retrying", "attempt", i+1, "err", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("redis connection failed after 10 retries: %w", err)
}
