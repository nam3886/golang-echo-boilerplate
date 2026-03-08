package events

import (
	"fmt"
	"log/slog"

	amqplib "github.com/rabbitmq/amqp091-go"
)

// DeclareDLQQueues ensures dead-letter queues exist for the given topics.
// Must be called at startup before subscribers begin consuming.
// Without these queues, nacked messages after retry exhaustion are silently
// dropped by RabbitMQ since the default exchange routes by queue name.
func DeclareDLQQueues(rabbitURL string, topics []string) error {
	conn, err := amqplib.Dial(rabbitURL)
	if err != nil {
		return fmt.Errorf("DLQ dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("DLQ channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	for _, topic := range topics {
		dlqName := topic + ".dlq"
		if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declaring queue %s: %w", dlqName, err)
		}
		slog.Debug("DLQ queue declared", "queue", dlqName)
	}
	return nil
}

// uniqueTopics returns a deduplicated slice of topics preserving order.
func uniqueTopics(topics []string) []string {
	seen := make(map[string]struct{}, len(topics))
	out := make([]string, 0, len(topics))
	for _, t := range topics {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}
