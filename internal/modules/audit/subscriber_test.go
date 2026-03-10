package audit

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// stubDBTX is a no-op DBTX for unit tests.
type stubDBTX struct {
	execErr error
}

func (s *stubDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, s.execErr
}

func (s *stubDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (s *stubDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return nil
}

func newTestHandler(execErr error) *Handler {
	db := &stubDBTX{execErr: execErr}
	return NewHandler(sqlcgen.New(db))
}

func newMsg(payload string) *message.Message {
	msg := message.NewMessage("test-uuid", []byte(payload))
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
