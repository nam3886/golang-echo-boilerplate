package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
	"github.com/golang-jwt/jwt/v5"
)

// stubClaims builds a minimal TokenClaims for testing.
func stubClaims(tokenID, userID string, expiresAt time.Time) *auth.TokenClaims {
	return &auth.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID: userID,
		Role:   "member",
	}
}

func TestLogoutHandler_Success_BlacklistsToken(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(bl, bus)

	claims := stubClaims("test-jti-001", "user-1", time.Now().Add(15*time.Minute))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	blacklisted, err := bl.IsBlacklisted(context.Background(), "test-jti-001")
	if err != nil {
		t.Fatalf("IsBlacklisted error: %v", err)
	}
	if !blacklisted {
		t.Error("expected token to be blacklisted after logout")
	}
}

func TestLogoutHandler_AlreadyExpiredToken_NoError(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(bl, bus)

	// Expired tokens: handler calls Blacklist with past expiry — no error expected.
	claims := stubClaims("expired-jti", "user-1", time.Now().Add(-1*time.Second))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("expected no error for expired token, got %v", err)
	}
}

func TestLogoutHandler_NilClaims_ReturnsUnauthorized(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(bl, bus)

	err := h.Handle(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claims")
	}
	if !isUnauthorized(err) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLogoutHandler_EventPublishFailure_DoesNotFail(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bus := events.NewEventBus(&testutil.FailPublisher{})
	h := NewLogoutHandler(bl, bus)

	claims := stubClaims("test-jti-002", "user-1", time.Now().Add(15*time.Minute))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("publish failure must not propagate: %v", err)
	}
}

func TestLogoutHandler_BlacklistWriteFailure_ReturnsError(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bl.BlacklistErr = fmt.Errorf("blacklist write failed")
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(bl, bus)

	claims := stubClaims("test-jti-fail", "user-1", time.Now().Add(15*time.Minute))

	err := h.Handle(context.Background(), claims)
	if err == nil {
		t.Fatal("expected error when blacklist write fails, got nil")
	}
}

func TestLogoutHandler_NilPanics(t *testing.T) {
	bl := testutil.NewStubBlacklister()
	bus := events.NewEventBus(&testutil.NoopPublisher{})

	assertPanics(t, "nil bl", func() { NewLogoutHandler(nil, bus) })
	assertPanics(t, "nil bus", func() { NewLogoutHandler(bl, nil) })
}

// isUnauthorized checks if err matches the UNAUTHENTICATED domain error code.
func isUnauthorized(err error) bool {
	var de *sharederr.DomainError
	if errors.As(err, &de) {
		return de.Code == sharederr.CodeUnauthenticated
	}
	return false
}
