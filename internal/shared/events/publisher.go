package events

import "context"

// EventPublisher is the interface the application layer depends on for publishing domain events.
// Concrete implementations (EventBus, test stubs) satisfy this interface.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, event any) error
}
