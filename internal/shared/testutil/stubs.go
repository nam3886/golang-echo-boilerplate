package testutil

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
)

// StubHasher returns a deterministic hash for testing.
type StubHasher struct{}

// Hash returns a deterministic hashed value for testing.
func (s *StubHasher) Hash(password string) (string, error) { return "hashed_" + password, nil }

// Verify always returns true for testing.
func (s *StubHasher) Verify(_, _ string) (bool, error) { return true, nil }

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

// CapturingPublisher records the last published topic and payload.
type CapturingPublisher struct {
	Topic   string
	Payload []byte
}

// Publish records the topic and first message payload.
func (r *CapturingPublisher) Publish(topic string, msgs ...*message.Message) error {
	r.Topic = topic
	if len(msgs) > 0 {
		r.Payload = msgs[0].Payload
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
