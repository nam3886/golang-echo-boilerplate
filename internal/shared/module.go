package shared

import (
	"context"
	"log/slog"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/database"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/observability"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
)

// Module provides all shared infrastructure to the Fx container.
var Module = fx.Module("shared",
	fx.Provide(config.Load),
	fx.Provide(database.NewPostgresPool),
	fx.Provide(database.NewRedisClient),
	fx.Provide(observability.NewLogger),
	fx.Provide(observability.NewTracerProvider),
	fx.Provide(observability.NewMeterProvider),
	fx.Invoke(registerOTelShutdown),
	fx.Invoke(registerDBShutdown),
)

// registerOTelShutdown ensures OTel providers flush pending data on shutdown.
func registerOTelShutdown(lc fx.Lifecycle, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			if err := mp.Shutdown(ctx); err != nil {
				slog.WarnContext(ctx, "meter provider shutdown error", "err", err)
			}
			if err := tp.Shutdown(ctx); err != nil {
				slog.ErrorContext(ctx, "tracer shutdown failed", "err", err)
			}
			return nil
		},
	})
}

// registerDBShutdown closes Postgres pool and Redis client on shutdown.
func registerDBShutdown(lc fx.Lifecycle, pool *pgxpool.Pool, rdb *redis.Client) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			slog.InfoContext(ctx, "postgres pool closed")
			if err := rdb.Close(); err != nil {
				slog.WarnContext(ctx, "redis close error", "err", err)
			} else {
				slog.InfoContext(ctx, "redis client closed")
			}
			return nil
		},
	})
}
