package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gnha/gnha-services/internal/modules/audit"
	"github.com/gnha/gnha-services/internal/modules/notification"
	"github.com/gnha/gnha-services/internal/modules/user"
	"github.com/gnha/gnha-services/internal/shared"
	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/gnha/gnha-services/internal/shared/events"
	appmw "github.com/gnha/gnha-services/internal/shared/middleware"
	"github.com/gnha/gnha-services/internal/shared/search"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Version is injected at build time via -ldflags="-X main.Version=$(git describe --tags --always)".
var Version config.AppVersion = "dev"

func main() {
	fx.New(
		fx.Supply(Version),
		shared.Module,
		fx.Provide(auth.NewPasswordHasher),
		fx.Provide(newEcho),
		// Search (optional — no-op when ELASTICSEARCH_URL is empty)
		search.Module,
		// Modules
		user.Module,
		audit.Module,
		notification.Module,
		// ADD_MODULE_HERE

		// Infrastructure
		events.Module,
		fx.Invoke(startServer),
	).Run()
}

func newEcho(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client, esClient *search.Client) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Health endpoints registered BEFORE middleware to avoid rate limiting.
	// Liveness — always OK (process is running)
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	// Readiness — checks DB + Redis + optional ES connectivity
	e.GET("/readyz", func(c echo.Context) error {
		ctx := c.Request().Context()
		if err := pool.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "postgres unavailable"})
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "redis unavailable"})
		}
		if esClient != nil {
			if err := esClient.HealthCheck(ctx); err != nil {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "elasticsearch unavailable"})
			}
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})

	// Full middleware chain
	appmw.SetupMiddleware(e, cfg, rdb)

	// Swagger UI (dev/staging only)
	appmw.MountSwagger(e, cfg)

	return e
}

func startServer(lc fx.Lifecycle, e *echo.Echo, cfg *config.Config, shutdowner fx.Shutdowner) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			addr := fmt.Sprintf(":%d", cfg.Port)
			slog.Info("server starting", "addr", addr, "env", cfg.AppEnv)
			go func() {
				if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
					slog.Error("server error", "err", err)
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("server shutting down")
			return e.Shutdown(ctx)
		},
	})
}
