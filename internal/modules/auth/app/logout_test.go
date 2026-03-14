package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
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
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(auth.NewRedisBlacklister(rdb), bus)

	claims := stubClaims("test-jti-001", "user-1", time.Now().Add(15*time.Minute))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the token ID was added to Redis.
	blacklisted, err := auth.IsBlacklisted(context.Background(), rdb, "test-jti-001")
	if err != nil {
		t.Fatalf("IsBlacklisted error: %v", err)
	}
	if !blacklisted {
		t.Error("expected token to be blacklisted after logout")
	}
}

func TestLogoutHandler_AlreadyExpiredToken_NoError(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(auth.NewRedisBlacklister(rdb), bus)

	// Expired tokens should not error — BlacklistToken is a no-op for expired tokens.
	claims := stubClaims("expired-jti", "user-1", time.Now().Add(-1*time.Second))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("expected no error for expired token, got %v", err)
	}
}

func TestLogoutHandler_NilClaims_ReturnsUnauthorized(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(auth.NewRedisBlacklister(rdb), bus)

	err := h.Handle(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claims")
	}
	if !isUnauthorized(err) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLogoutHandler_EventPublishFailure_DoesNotFail(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	bus := events.NewEventBus(&testutil.FailPublisher{})
	h := NewLogoutHandler(auth.NewRedisBlacklister(rdb), bus)

	claims := stubClaims("test-jti-002", "user-1", time.Now().Add(15*time.Minute))

	if err := h.Handle(context.Background(), claims); err != nil {
		t.Fatalf("publish failure must not propagate: %v", err)
	}
}

func TestLogoutHandler_RedisWriteFailure_ReturnsError(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	bus := events.NewEventBus(&testutil.NoopPublisher{})
	h := NewLogoutHandler(auth.NewRedisBlacklister(rdb), bus)

	claims := stubClaims("test-jti-fail", "user-1", time.Now().Add(15*time.Minute))

	// Close Redis before the blacklist write to simulate failure.
	mr.Close()

	err := h.Handle(context.Background(), claims)
	if err == nil {
		t.Fatal("expected error when Redis is unavailable, got nil")
	}
}

func TestLogoutHandler_NilPanics(t *testing.T) {
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	assertPanics(t, "nil bl", func() { NewLogoutHandler(nil, bus) })
	assertPanics(t, "nil bus", func() { NewLogoutHandler(auth.NewRedisBlacklister(rdb), nil) })
}

// isUnauthorized checks if err matches the UNAUTHENTICATED domain error code.
func isUnauthorized(err error) bool {
	var de *sharederr.DomainError
	if errors.As(err, &de) {
		return de.Code == sharederr.CodeUnauthenticated
	}
	return false
}
