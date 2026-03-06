package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v3/pkg/amqp"
	amqplib "github.com/rabbitmq/amqp091-go"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// EventBus wraps Watermill publisher for domain event publishing.
type EventBus struct {
	publisher message.Publisher
}

// newSubscriberConfig builds an AMQP config with dead-letter exchange (DLX) routing.
// When Watermill retries are exhausted and the message is nacked, RabbitMQ routes it
// to "{topic}.dlq" via the default exchange (routing key = topic name).
func newSubscriberConfig(rabbitURL string) amqp.Config {
	cfg := amqp.NewDurableQueueConfig(rabbitURL)
	// RabbitMQ dead-letter exchange: use default exchange ("").
	// The dead-letter routing key defaults to the original routing key (= topic),
	// so a message from queue "user.created" lands in queue "user.created.dlq"
	// if that queue is declared separately with the same routing key suffix.
	cfg.Queue.Arguments = amqplib.Table{
		"x-dead-letter-exchange": "",
	}
	return cfg
}

// NewPublisher creates a Watermill AMQP publisher.
func NewPublisher(cfg *config.Config) (message.Publisher, error) {
	amqpCfg := amqp.NewDurableQueueConfig(cfg.RabbitURL)
	pub, err := amqp.NewPublisher(amqpCfg, watermill.NewSlogLogger(slog.Default()))
	if err != nil {
		return nil, fmt.Errorf("creating AMQP publisher: %w", err)
	}
	return pub, nil
}

// NewSubscriber creates a Watermill AMQP subscriber with DLX routing configured.
func NewSubscriber(cfg *config.Config) (message.Subscriber, error) {
	amqpCfg := newSubscriberConfig(cfg.RabbitURL)
	sub, err := amqp.NewSubscriber(amqpCfg, watermill.NewSlogLogger(slog.Default()))
	if err != nil {
		return nil, fmt.Errorf("creating AMQP subscriber: %w", err)
	}
	return sub, nil
}

// NewEventBus creates an EventBus wrapping a publisher.
func NewEventBus(publisher message.Publisher) *EventBus {
	return &EventBus{publisher: publisher}
}

// Publish marshals and publishes a domain event with OTel trace propagation.
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

	slog.Debug("event published", "topic", topic, "id", msg.UUID)
	return nil
}
