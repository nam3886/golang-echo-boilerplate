//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

func TestBlacklist_BlacklistAndCheck(t *testing.T) {
	rdb := testutil.NewTestRedis(t)
	ctx := context.Background()
	jti := "test-jti-1"
	expiry := time.Now().Add(5 * time.Minute)

	if err := auth.BlacklistToken(ctx, rdb, jti, expiry); err != nil {
		t.Fatalf("BlacklistToken: %v", err)
	}

	blacklisted, err := auth.IsBlacklisted(ctx, rdb, jti)
	if err != nil {
		t.Fatalf("IsBlacklisted: %v", err)
	}
	if !blacklisted {
		t.Error("expected jti to be blacklisted")
	}
}

func TestBlacklist_NotBlacklisted(t *testing.T) {
	rdb := testutil.NewTestRedis(t)
	ctx := context.Background()

	blacklisted, err := auth.IsBlacklisted(ctx, rdb, "unknown-jti")
	if err != nil {
		t.Fatalf("IsBlacklisted: %v", err)
	}
	if blacklisted {
		t.Error("expected unknown jti to not be blacklisted")
	}
}

func TestBlacklist_AlreadyExpiredSkipped(t *testing.T) {
	rdb := testutil.NewTestRedis(t)
	ctx := context.Background()
	jti := "expired-jti"
	pastExpiry := time.Now().Add(-1 * time.Minute)

	// BlacklistToken returns nil for already-expired tokens (no-op)
	if err := auth.BlacklistToken(ctx, rdb, jti, pastExpiry); err != nil {
		t.Fatalf("BlacklistToken: %v", err)
	}

	blacklisted, err := auth.IsBlacklisted(ctx, rdb, jti)
	if err != nil {
		t.Fatalf("IsBlacklisted: %v", err)
	}
	if blacklisted {
		t.Error("expected expired token to NOT be blacklisted (no-op path)")
	}
}

func TestBlacklist_TTLRespectsExpiry(t *testing.T) {
	rdb := testutil.NewTestRedis(t)
	ctx := context.Background()
	jti := "short-lived-jti"
	expiry := time.Now().Add(1 * time.Second)

	if err := auth.BlacklistToken(ctx, rdb, jti, expiry); err != nil {
		t.Fatalf("BlacklistToken: %v", err)
	}

	// Confirm it is blacklisted immediately
	blacklisted, err := auth.IsBlacklisted(ctx, rdb, jti)
	if err != nil {
		t.Fatalf("IsBlacklisted before TTL: %v", err)
	}
	if !blacklisted {
		t.Fatal("expected jti to be blacklisted before TTL expiry")
	}

	// Wait for Redis key to expire — intentionally slow (real Redis TTL, not a mock).
	// 1100ms gives the 1s TTL time to expire plus a 100ms buffer for clock skew.
	time.Sleep(1100 * time.Millisecond)

	blacklisted, err = auth.IsBlacklisted(ctx, rdb, jti)
	if err != nil {
		t.Fatalf("IsBlacklisted after TTL: %v", err)
	}
	if blacklisted {
		t.Error("expected jti to no longer be blacklisted after TTL expiry")
	}
}
