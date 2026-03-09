package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/gnha/gnha-services/internal/shared/retry"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client with retry logic.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}
	poolSize := 10 * runtime.NumCPU()
	if poolSize > 100 {
		poolSize = 100
	}
	opt.PoolSize = poolSize
	opt.MinIdleConns = 5

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err = retry.Connect(ctx, "redis", 10, func() (struct{}, error) {
		if pingErr := rdb.Ping(ctx).Err(); pingErr != nil {
			return struct{}{}, pingErr
		}
		return struct{}{}, nil
	})
	if err != nil {
		_ = rdb.Close() // prevent goroutine/connection leak
		return nil, err
	}
	slog.Info("redis connected", "addr", opt.Addr)
	return rdb, nil
}
