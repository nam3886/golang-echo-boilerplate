package audit

import (
	"context"
	"fmt"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
)

// stubAuditWriter stubs the auditWriter interface at the correct layer.
// Stubbing pgx.DBTX instead would cause a nil-pointer panic: sqlc calls
// QueryRow().Scan() and a nil pgx.Row panics on Scan.
type stubAuditWriter struct {
	err error
}

func (s *stubAuditWriter) CreateAuditLog(_ context.Context, _ sqlcgen.CreateAuditLogParams) error {
	return s.err
}

func newTestHandler(execErr error) *Handler {
	return NewHandler(&stubAuditWriter{err: execErr})
}

func newMsg(payload string) *message.Message {
	msg := message.NewMessage("550e8400-e29b-41d4-a716-446655440000", []byte(payload))
	msg.SetContext(context.Background())
	return msg
}

func TestHandleUserCreated_ValidPayload(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002","email":"user@example.com","name":"Test User","role":"member"}`)
	err := h.HandleUserCreated(msg)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHandleUserCreated_InvalidJSON(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`not-json`)
	err := h.HandleUserCreated(msg)
	// Invalid JSON should ack (return nil), not block the queue.
	if err != nil {
		t.Errorf("expected nil error on bad payload, got %v", err)
	}
}

func TestHandleUserCreated_InvalidUserID(t *testing.T) {
	h := newTestHandler(nil)
	// Valid JSON but user_id is not a valid UUID.
	msg := newMsg(`{"user_id":"not-a-uuid","actor_id":"00000000-0000-0000-0000-000000000002"}`)
	err := h.HandleUserCreated(msg)
	if err != nil {
		t.Errorf("expected nil error on invalid user_id, got %v", err)
	}
}

func TestHandleUserUpdated_ValidPayload(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002"}`)
	err := h.HandleUserUpdated(msg)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHandleUserUpdated_InvalidJSON(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`{invalid}`)
	err := h.HandleUserUpdated(msg)
	if err != nil {
		t.Errorf("expected nil error on bad payload, got %v", err)
	}
}

func TestHandleUserDeleted_ValidPayload(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002"}`)
	err := h.HandleUserDeleted(msg)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHandleUserDeleted_InvalidJSON(t *testing.T) {
	h := newTestHandler(nil)
	msg := newMsg(`bad`)
	err := h.HandleUserDeleted(msg)
	if err != nil {
		t.Errorf("expected nil error on bad payload, got %v", err)
	}
}

func TestHandleUserCreated_DBError(t *testing.T) {
	dbErr := fmt.Errorf("connection reset by peer")
	h := newTestHandler(dbErr)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002","email":"user@example.com","name":"Test User","role":"member"}`)
	err := h.HandleUserCreated(msg)
	if err == nil {
		t.Error("expected DB error to propagate for retry, got nil")
	}
}

func TestHandleUserUpdated_DBError(t *testing.T) {
	dbErr := fmt.Errorf("deadlock detected")
	h := newTestHandler(dbErr)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002"}`)
	err := h.HandleUserUpdated(msg)
	if err == nil {
		t.Error("expected DB error to propagate for retry, got nil")
	}
}

func TestHandleUserDeleted_DBError(t *testing.T) {
	dbErr := fmt.Errorf("disk full")
	h := newTestHandler(dbErr)
	msg := newMsg(`{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002"}`)
	err := h.HandleUserDeleted(msg)
	if err == nil {
		t.Error("expected DB error to propagate for retry, got nil")
	}
}
