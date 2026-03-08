package cron

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

// Scheduler wraps robfig/cron with Redis distributed locking.
type Scheduler struct {
	cron *cron.Cron
	rdb  *redis.Client
}

// NewScheduler creates a cron scheduler with second-level precision.
func NewScheduler(rdb *redis.Client) *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		rdb:  rdb,
	}
}

// unlockScript deletes the key only if we still own it (value matches).
const unlockScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`

// AddJob registers a cron job with a distributed lock to prevent duplicates.
func (s *Scheduler) AddJob(spec, name string, fn func(ctx context.Context) error) error {
	_, err := s.cron.AddFunc(spec, func() {
		ctx := context.Background()
		lockKey := "cron:" + name
		lockVal := uuid.NewString()

		// Acquire distributed lock with unique token
		locked, err := s.rdb.SetArgs(ctx, lockKey, lockVal, redis.SetArgs{
			Mode: "NX",
			TTL:  5 * time.Minute,
		}).Result()
		// locked is "OK" when the NX lock was acquired
		if err != nil || locked != "OK" {
			return // Another instance has the lock
		}
		defer func() {
			if err := s.rdb.Eval(ctx, unlockScript, []string{lockKey}, lockVal).Err(); err != nil {
				slog.Warn("cron: failed to unlock", "job", name, "key", lockKey, "err", err)
			}
		}()

		if err := fn(ctx); err != nil {
			slog.Error("cron job failed", "job", name, "err", err)
		} else {
			slog.Info("cron job completed", "job", name)
		}
	})
	return err
}

// Start begins the cron scheduler as part of Fx lifecycle.
func Start(lc fx.Lifecycle, s *Scheduler) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			s.cron.Start()
			slog.Info("cron scheduler started")
			return nil
		},
		OnStop: func(_ context.Context) error {
			<-s.cron.Stop().Done()
			slog.Info("cron scheduler stopped")
			return nil
		},
	})
}
