package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

type testEvent struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

func TestEventBus_Publish_Success(t *testing.T) {
	cap := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(cap)

	ev := testEvent{UserID: "abc-123", Name: "Alice"}
	if err := bus.Publish(context.Background(), "user.created", ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cap.Messages) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(cap.Messages))
	}

	msg := cap.Messages[0]
	if msg.Topic != "user.created" {
		t.Errorf("expected topic %q, got %q", "user.created", msg.Topic)
	}

	var decoded testEvent
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded.UserID != ev.UserID || decoded.Name != ev.Name {
		t.Errorf("payload mismatch: got %+v, want %+v", decoded, ev)
	}
}

func TestEventBus_Publish_NilPayload(t *testing.T) {
	cap := &testutil.CapturingPublisher{}
	bus := events.NewEventBus(cap)

	// nil marshals to JSON "null" — should succeed without error.
	if err := bus.Publish(context.Background(), "user.noop", nil); err != nil {
		t.Fatalf("unexpected error publishing nil payload: %v", err)
	}

	if len(cap.Messages) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(cap.Messages))
	}
}

func TestEventBus_Publish_PublisherError(t *testing.T) {
	fail := &testutil.FailPublisher{}
	bus := events.NewEventBus(fail)

	err := bus.Publish(context.Background(), "user.created", testEvent{UserID: "x"})
	if err == nil {
		t.Fatal("expected error from failing publisher, got nil")
	}
}
