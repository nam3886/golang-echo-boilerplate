package events

import (
	"context"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.uber.org/fx"
)

// Module provides the event infrastructure to the Fx container.
var Module = fx.Module("events",
	fx.Provide(NewPublisher),
	fx.Provide(NewSubscriberFactory),
	fx.Provide(
		fx.Annotate(
			NewEventBus,
			fx.As(new(EventPublisher)),
		),
	),
	fx.Provide(NewRouter),
	fx.Invoke(registerAMQPShutdown),
	fx.Invoke(StartRouter),
)

// registerAMQPShutdown closes the publisher on Fx shutdown.
// NOTE: subscriber is owned by the Watermill router — router.Close() in
// StartRouter handles its lifecycle. Closing it here would cause a
// double-close panic because Fx shutdown hook ordering is non-deterministic.
func registerAMQPShutdown(lc fx.Lifecycle, pub message.Publisher) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			if err := pub.Close(); err != nil {
				slog.WarnContext(ctx, "amqp publisher close error", "err", err)
			} else {
				slog.InfoContext(ctx, "amqp publisher closed")
			}
			return nil
		},
	})
}
