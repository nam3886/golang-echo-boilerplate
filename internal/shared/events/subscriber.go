package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/gnha/gnha-services/internal/shared/config"
	"go.uber.org/fx"
)

// RouterParams holds dependencies for the Watermill router.
type RouterParams struct {
	fx.In
	Config     *config.Config
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
// It also declares DLQ queues for all registered topics so that nacked
// messages after retry exhaustion are routed to "{topic}.dlq" instead of
// being silently dropped by RabbitMQ.
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

	// Declare DLQ queues for all registered topics.
	// DLQs are a safety net -- failure to declare is non-fatal (warn only).
	topics := make([]string, 0, len(params.Handlers))
	for _, h := range params.Handlers {
		topics = append(topics, h.Topic)
	}
	if err := DeclareDLQQueues(params.Config.RabbitURL, uniqueTopics(topics)); err != nil {
		slog.Warn("failed to declare DLQ queues; dead-lettered messages may be dropped",
			"err", err)
	}

	return router, nil
}

// StartRouter starts the Watermill router as part of Fx lifecycle.
// On fatal router error, triggers Fx shutdown with exit code 1.
func StartRouter(lc fx.Lifecycle, router *message.Router, shutdowner fx.Shutdowner) {
	ctx := context.Background()
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
			return router.Close()
		},
	})
}
