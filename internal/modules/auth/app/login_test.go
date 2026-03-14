package app

import (
	"context"
	"errors"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

// stubLookup implements auth.CredentialLookup for testing.
type stubLookup struct {
	userID   string
	hashPwd  string
	role     string
	err      error
}

func (s *stubLookup) GetByEmail(_ context.Context, _ string) (string, string, string, error) {
	return s.userID, s.hashPwd, s.role, s.err
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		// Minimal config for unit tests — infra URLs are not needed.
		//nolint:gosec // hardcoded secret is test-only
		return &config.Config{
			AppName:      "test",
			JWTSecret:    "test-secret-32-bytes-long-padding!",
			JWTAccessTTL: 900_000_000_000, // 15m in nanoseconds
		}
	}
	return cfg
}

func TestLoginHandler_Success(t *testing.T) {
	lookup := &stubLookup{
		userID:  "00000000-0000-0000-0000-000000000001",
		hashPwd: "hashed_password123",
		role:    "member",
	}
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)

	result, err := h.Handle(context.Background(), LoginCmd{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if result.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", result.ExpiresIn)
	}
}

func TestLoginHandler_UserNotFound_ReturnsInvalidCredentials(t *testing.T) {
	lookup := &stubLookup{err: sharederr.ErrNotFound()}
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)

	_, err := h.Handle(context.Background(), LoginCmd{
		Email:    "missing@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCredentials()) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginHandler_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	lookup := &stubLookup{
		userID:  "00000000-0000-0000-0000-000000000001",
		hashPwd: "hashed_correctpassword",
		role:    "member",
	}
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)

	_, err := h.Handle(context.Background(), LoginCmd{
		Email:    "user@example.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCredentials()) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginHandler_LookupError_PropagatesError(t *testing.T) {
	lookup := &stubLookup{err: errors.New("db connection lost")}
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)

	_, err := h.Handle(context.Background(), LoginCmd{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error from lookup failure")
	}
	// Must NOT be masked as invalid credentials
	if errors.Is(err, ErrInvalidCredentials()) {
		t.Error("db error should not be masked as invalid credentials")
	}
}

func TestLoginHandler_EventPublishFailure_DoesNotFail(t *testing.T) {
	lookup := &stubLookup{
		userID:  "00000000-0000-0000-0000-000000000001",
		hashPwd: "hashed_password123",
		role:    "member",
	}
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.FailPublisher{})
	h := NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)

	result, err := h.Handle(context.Background(), LoginCmd{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("publish failure must not propagate: %v", err)
	}
	if result.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestLoginHandler_NilPanics(t *testing.T) {
	cfg := testConfig(t)
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	lookup := &stubLookup{}

	assertPanics(t, "nil lookup", func() { NewLoginHandler(nil, &testutil.StubHasher{}, cfg, bus) })
	assertPanics(t, "nil hasher", func() { NewLoginHandler(lookup, nil, cfg, bus) })
	assertPanics(t, "nil cfg", func() { NewLoginHandler(lookup, &testutil.StubHasher{}, nil, bus) })
	assertPanics(t, "nil bus", func() { NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, nil) })
}

func assertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", label)
		}
	}()
	fn()
}
