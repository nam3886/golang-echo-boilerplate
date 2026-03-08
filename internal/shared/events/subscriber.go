package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"go.uber.org/fx"
)

// RouterParams holds dependencies for the Watermill router.
type RouterParams struct {
	fx.In
	Subscriber message.Subscriber
	Handlers   []HandlerRegistration `group:"event_handlers"`
}

// HandlerRegistration describes how to register an event handler.
type HandlerRegistration struct {
	Name        string
	Topic       string
	HandlerFunc message.NoPublishHandlerFunc
}

// NewRouter creates and configures the Watermill message router.
func NewRouter(params RouterParams) (*message.Router, error) {
	router, err := message.NewRouter(message.RouterConfig{},
		watermill.NewSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, err
	}

	// Add middleware
	router.AddMiddleware(
		middleware.Recoverer,
		middleware.Retry{
			MaxRetries:          3,
			InitialInterval:     time.Second,
			RandomizationFactor: 0.5,
		}.Middleware,
	)

	// Register handlers from all modules
	for _, h := range params.Handlers {
		router.AddConsumerHandler(h.Name, h.Topic, params.Subscriber, h.HandlerFunc)
	}

	return router, nil
}

// StartRouter starts the Watermill router as part of Fx lifecycle.
// On fatal router error, triggers Fx shutdown with exit code 1.
func StartRouter(lc fx.Lifecycle, router *message.Router, shutdowner fx.Shutdowner) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				if err := router.Run(ctx); err != nil {
					slog.Error("watermill router fatal error, initiating shutdown", "err", err)
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return router.Close()
		},
	})
}
