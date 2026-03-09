package testutil

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
)

// StubHasher returns a deterministic hash for testing.
type StubHasher struct{}

// Hash returns a deterministic hashed value for testing.
func (s *StubHasher) Hash(password string) (string, error) { return "hashed_" + password, nil }

// Verify returns true only when encoded matches the Hash output pattern.
func (s *StubHasher) Verify(password, encoded string) (bool, error) {
	return encoded == "hashed_"+password, nil
}

// FailHasher always returns an error from Hash.
type FailHasher struct{}

// Hash always returns an error simulating a hasher failure.
func (f *FailHasher) Hash(_ string) (string, error) { return "", fmt.Errorf("hasher unavailable") }

// Verify always returns false for testing.
func (f *FailHasher) Verify(_, _ string) (bool, error) { return false, nil }

// NoopPublisher discards all published messages.
type NoopPublisher struct{}

// Publish discards the message and returns nil.
func (p *NoopPublisher) Publish(_ string, _ ...*message.Message) error { return nil }

// Close is a no-op.
func (p *NoopPublisher) Close() error { return nil }

// CapturedMessage records a single published message.
type CapturedMessage struct {
	Topic   string
	Payload []byte
}

// CapturingPublisher records all published messages.
type CapturingPublisher struct {
	Messages []CapturedMessage
}

// Publish appends all message payloads (not just the first).
func (r *CapturingPublisher) Publish(topic string, msgs ...*message.Message) error {
	for _, msg := range msgs {
		r.Messages = append(r.Messages, CapturedMessage{Topic: topic, Payload: msg.Payload})
	}
	return nil
}

// Close is a no-op.
func (r *CapturingPublisher) Close() error { return nil }

// FailPublisher always returns an error from Publish.
type FailPublisher struct{}

// Publish always returns an error simulating a publisher failure.
func (f *FailPublisher) Publish(_ string, _ ...*message.Message) error {
	return fmt.Errorf("publisher unavailable")
}

// Close is a no-op.
func (f *FailPublisher) Close() error { return nil }
