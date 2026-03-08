package events

import (
	"fmt"
	"log/slog"

	amqplib "github.com/rabbitmq/amqp091-go"
)

// DeclareDLQQueues ensures the "dlx" exchange and dead-letter queues exist for
// the given topics. Must be called at startup before subscribers begin consuming.
// Each {topic}.dlq queue is bound to the "dlx" exchange with routing key = topic,
// so dead-lettered messages are correctly routed instead of looping back.
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

	// Declare a dedicated dead-letter exchange (direct type).
	if err := ch.ExchangeDeclare("dlx", "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declaring DLX exchange: %w", err)
	}

	for _, topic := range topics {
		dlqName := topic + ".dlq"
		if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declaring queue %s: %w", dlqName, err)
		}
		// Bind: DLX receives dead-lettered msg with routing key = topic → route to {topic}.dlq
		if err := ch.QueueBind(dlqName, topic, "dlx", false, nil); err != nil {
			return fmt.Errorf("binding queue %s to dlx: %w", dlqName, err)
		}
		slog.Debug("DLQ queue declared and bound", "queue", dlqName, "routing_key", topic)
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
