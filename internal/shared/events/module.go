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
	fx.Provide(NewSubscriber),
	fx.Provide(NewEventBus),
	fx.Provide(NewRouter),
	fx.Invoke(StartRouter),
	fx.Invoke(registerAMQPShutdown),
)

// registerAMQPShutdown closes publisher and subscriber on Fx shutdown.
func registerAMQPShutdown(lc fx.Lifecycle, pub message.Publisher, sub message.Subscriber) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			if err := pub.Close(); err != nil {
				slog.Warn("amqp publisher close error", "err", err)
			} else {
				slog.Info("amqp publisher closed")
			}
			if err := sub.Close(); err != nil {
				slog.Warn("amqp subscriber close error", "err", err)
			} else {
				slog.Info("amqp subscriber closed")
			}
			return nil
		},
	})
}
