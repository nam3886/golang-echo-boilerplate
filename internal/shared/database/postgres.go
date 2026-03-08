package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates a pgx connection pool with retry logic.
func NewPostgresPool(cfg *config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = cfg.DBMinConns
	poolCfg.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	var pool *pgxpool.Pool
	for i := range 10 {
		pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				slog.Info("postgres connected", "host", poolCfg.ConnConfig.Host)
				return pool, nil
			}
			pool.Close()
		}
		slog.Warn("postgres not ready, retrying", "attempt", i+1, "err", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("postgres connection failed after 10 retries: %w", err)
}
