package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v3/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/google/uuid"
	amqplib "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// EventBus wraps Watermill publisher for domain event publishing.
type EventBus struct {
	publisher message.Publisher
}

// NewPublisher creates a Watermill AMQP publisher using fanout exchange (PubSub pattern).
func NewPublisher(cfg *config.Config) (message.Publisher, error) {
	amqpCfg := amqp.NewDurablePubSubConfig(cfg.RabbitURL, nil)
	pub, err := amqp.NewPublisher(amqpCfg, watermill.NewSlogLogger(slog.Default()))
	if err != nil {
		return nil, fmt.Errorf("creating AMQP publisher: %w", err)
	}
	return pub, nil
}

// SubscriberFactory creates per-handler AMQP subscribers so each handler
// gets its own queue. This prevents round-robin message distribution
// when multiple handlers subscribe to the same topic.
type SubscriberFactory struct {
	rabbitURL string
}

// NewSubscriberFactory creates a factory for per-handler subscribers.
func NewSubscriberFactory(cfg *config.Config) *SubscriberFactory {
	return &SubscriberFactory{rabbitURL: cfg.RabbitURL}
}

// Create builds a new subscriber with a unique queue for the given handler and topic.
// Queue name = "{topic}_{handlerName}" via GenerateQueueNameTopicNameWithSuffix.
// DLX routing key is set to the topic so dead-lettered messages reach "{topic}.dlq"
// via the "dlx" direct exchange (which binds queues by topic routing key).
func (f *SubscriberFactory) Create(handlerName, topic string) (message.Subscriber, error) {
	amqpCfg := amqp.NewDurablePubSubConfig(f.rabbitURL,
		amqp.GenerateQueueNameTopicNameWithSuffix(handlerName))
	amqpCfg.Queue.Arguments = amqplib.Table{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": topic,
	}
	return amqp.NewSubscriber(amqpCfg, watermill.NewSlogLogger(slog.Default()))
}

// NewEventBus creates an EventBus wrapping a publisher.
func NewEventBus(publisher message.Publisher) *EventBus {
	return &EventBus{publisher: publisher}
}

// Publish marshals and publishes a domain event with OTel trace propagation.
//
// WARNING: Publish silently succeeds if the underlying Watermill publisher
// is closed. Callers should treat publish errors as non-fatal (fire-and-forget).
func (b *EventBus) Publish(ctx context.Context, topic string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	msg := message.NewMessage(uuid.NewString(), payload)
	// Propagate trace context into message metadata
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Metadata))
	msg.Metadata.Set("event_type", topic)

	if err := b.publisher.Publish(topic, msg); err != nil {
		return fmt.Errorf("publishing event %s: %w", topic, err)
	}

	slog.DebugContext(ctx, "event published", "topic", topic, "id", msg.UUID)
	return nil
}
