package notification

import "context"

// Sender is the interface for sending notifications.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}
