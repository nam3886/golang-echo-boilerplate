package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/gnha/gnha-services/internal/shared/retry"
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

	pool, err := retry.Connect(ctx, "postgres", 10, func() (*pgxpool.Pool, error) {
		p, err := pgxpool.NewWithConfig(ctx, poolCfg)
		if err != nil {
			return nil, err
		}
		if err := p.Ping(ctx); err != nil {
			p.Close()
			return nil, err
		}
		return p, nil
	})
	if err != nil {
		return nil, err
	}
	slog.Info("postgres connected", "host", poolCfg.ConnConfig.Host)
	return pool, nil
}
