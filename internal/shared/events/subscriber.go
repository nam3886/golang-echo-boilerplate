package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/fx"
)

// RouterParams holds dependencies for the Watermill router.
type RouterParams struct {
	fx.In
	Config   *config.Config
	Factory  *SubscriberFactory
	Handlers []HandlerRegistration `group:"event_handlers"`
}

// HandlerRegistration describes how to register an event handler.
type HandlerRegistration struct {
	Name        string
	Topic       string
	HandlerFunc message.NoPublishHandlerFunc
}

// otelExtractMiddleware extracts OTel trace context from message metadata,
// restoring distributed trace continuity across event boundaries.
func otelExtractMiddleware(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		ctx := otel.GetTextMapPropagator().Extract(msg.Context(), propagation.MapCarrier(msg.Metadata))
		msg.SetContext(ctx)
		return h(msg)
	}
}

// NewRouter creates and configures the Watermill message router.
// Each handler gets its own subscriber (and thus its own queue) via the
// SubscriberFactory, ensuring every handler receives all messages on its topic
// instead of round-robin sharing.
func NewRouter(params RouterParams) (*message.Router, error) {
	router, err := message.NewRouter(message.RouterConfig{},
		watermill.NewSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, err
	}

	// Add middleware
	router.AddMiddleware(
		otelExtractMiddleware,
		middleware.Recoverer,
		middleware.Retry{
			MaxRetries:          3,
			InitialInterval:     time.Second,
			Multiplier:          2,
			MaxInterval:         10 * time.Second,
			RandomizationFactor: 0.5,
		}.Middleware,
	)

	// Register handlers — each gets its own subscriber with a unique queue.
	for _, h := range params.Handlers {
		sub, err := params.Factory.Create(h.Name, h.Topic)
		if err != nil {
			return nil, fmt.Errorf("creating subscriber for %s: %w", h.Name, err)
		}
		router.AddConsumerHandler(h.Name, h.Topic, sub, h.HandlerFunc)
	}

	// Declare DLQ queues for all registered topics.
	// DLQs are a safety net — failure to declare is an error requiring attention.
	topics := make([]string, 0, len(params.Handlers))
	for _, h := range params.Handlers {
		topics = append(topics, h.Topic)
	}
	if err := DeclareDLQQueues(context.Background(), params.Config.RabbitURL, uniqueTopics(topics)); err != nil {
		return nil, fmt.Errorf("declaring DLQ queues: %w", err)
	}

	return router, nil
}

// StartRouter starts the Watermill router as part of Fx lifecycle.
// Uses a cancellable context so the router stops cleanly on shutdown.
// On fatal router error, triggers Fx shutdown with exit code 1.
func StartRouter(lc fx.Lifecycle, router *message.Router, shutdowner fx.Shutdowner) {
	var cancel context.CancelFunc
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context
			// nolint:gosec // cancel is called in OnStop hook below
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				if err := router.Run(ctx); err != nil {
					slog.Error("watermill router fatal error, initiating shutdown",
						"module", "events", "operation", "RouterRun",
						"error_code", "router_fatal", "retryable", false, "err", err)
					// Only trigger Fx shutdown — do NOT call cancel() or router.Close() here.
					// OnStop is the single shutdown coordinator to avoid racing with this goroutine.
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			// Single shutdown coordinator: cancel context first (signals router.Run to stop),
			// then close the router. No other goroutine should call these.
			cancel()
			return router.Close()
		},
	})
}
