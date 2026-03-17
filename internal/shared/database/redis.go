package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/retry"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client with retry logic.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}
	poolSize := min(10*runtime.NumCPU(), 100)
	opt.PoolSize = poolSize
	opt.MinIdleConns = 5

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = retry.Do(ctx, "redis", 10, func() error {
		return rdb.Ping(ctx).Err()
	})
	if err != nil {
		_ = rdb.Close() // prevent goroutine/connection leak
		return nil, err
	}
	slog.Info("redis connected", "module", "infra", "operation", "startup", "addr", opt.Addr)
	return rdb, nil
}
