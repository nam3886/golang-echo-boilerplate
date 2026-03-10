package search

import (
	"context"
	"log/slog"

	"go.uber.org/fx"
)

// Module provides the optional Elasticsearch client to the Fx container.
var Module = fx.Module("search",
	fx.Provide(NewClient),
	fx.Invoke(registerShutdown),
)

func registerShutdown(lc fx.Lifecycle, client *Client) {
	if client == nil {
		return
	}
	// go-elasticsearch client has no Close() method.
	// Log shutdown for observability; no resources to release.
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			slog.Info("elasticsearch client shutdown")
			return nil
		},
	})
}
