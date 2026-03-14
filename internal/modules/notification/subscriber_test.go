package notification

import (
	"context"
	"errors"
	"net/textproto"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// stubSender is a configurable Sender stub for unit tests.
type stubSender struct {
	err error
}

func (s *stubSender) Send(_ context.Context, _, _, _ string) error {
	return s.err
}

// newTestRedis starts an in-memory Redis for unit tests.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newTestHandler(t *testing.T, senderErr error) *Handler {
	t.Helper()
	return NewHandler(&stubSender{err: senderErr}, newTestRedis(t))
}

func newMsg(payload string) *message.Message {
	msg := message.NewMessage("550e8400-e29b-41d4-a716-446655440000", []byte(payload))
	msg.SetContext(context.Background())
	return msg
}

const validUserCreatedPayload = `{"user_id":"00000000-0000-0000-0000-000000000001","actor_id":"00000000-0000-0000-0000-000000000002","email":"user@example.com","name":"Test User","role":"member"}`

func TestHandleUserCreated_ValidPayload(t *testing.T) {
	h := newTestHandler(t, nil)
	err := h.HandleUserCreated(newMsg(validUserCreatedPayload))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHandleUserCreated_InvalidJSON(t *testing.T) {
	h := newTestHandler(t, nil)
	// Invalid JSON must ack (return nil) — schema mismatch is permanent.
	err := h.HandleUserCreated(newMsg(`not-json`))
	if err != nil {
		t.Errorf("expected nil error on invalid JSON, got %v", err)
	}
}

func TestHandleUserCreated_SenderError_Transient(t *testing.T) {
	// A transient error (non-5xx) should be returned so Watermill retries.
	h := newTestHandler(t, errors.New("connection refused"))
	err := h.HandleUserCreated(newMsg(validUserCreatedPayload))
	if err == nil {
		t.Error("expected non-nil error for transient sender failure")
	}
}

func TestHandleUserCreated_SenderError_Permanent(t *testing.T) {
	// A permanent SMTP 5xx error should ack (return nil) — retrying won't help.
	h := newTestHandler(t, &textproto.Error{Code: 550, Msg: "user unknown"})
	err := h.HandleUserCreated(newMsg(validUserCreatedPayload))
	if err != nil {
		t.Errorf("expected nil error for permanent SMTP error, got %v", err)
	}
}

func TestIsPermanentSMTPError(t *testing.T) {
	cases := []struct {
		err       error
		permanent bool
	}{
		{&textproto.Error{Code: 550, Msg: "mailbox not found"}, true},
		{&textproto.Error{Code: 551, Msg: "user not local"}, true},
		{&textproto.Error{Code: 554, Msg: "transaction failed"}, true},
		{errors.New("connection refused"), false},
		{errors.New("timeout after 550ms"), false}, // plain string must not match
	}
	for _, tc := range cases {
		got := isPermanentSMTPError(tc.err)
		if got != tc.permanent {
			t.Errorf("isPermanentSMTPError(%v) = %v, want %v", tc.err, got, tc.permanent)
		}
	}
}
