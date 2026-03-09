package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/config"
)

func testJWTConfig() *config.Config {
	return &config.Config{
		AppName:      "test-app",
		JWTSecret:    "test-secret-must-be-at-least-32-chars!!",
		JWTAccessTTL: 15 * time.Minute,
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	cfg := testJWTConfig()
	perms := []string{"user:read", "user:write"}

	token, err := auth.GenerateAccessToken(cfg, "user-123", "admin", perms)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := auth.ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", claims.Role)
	}
	if len(claims.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(claims.Permissions))
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	cfg := testJWTConfig()
	cfg.JWTAccessTTL = -1 * time.Second

	token, err := auth.GenerateAccessToken(cfg, "user-123", "member", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = auth.ValidateAccessToken(cfg, token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	cfg := testJWTConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-123", "member", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	wrongCfg := testJWTConfig()
	wrongCfg.JWTSecret = "wrong-secret-must-be-at-least-32-chars!!"

	_, err = auth.ValidateAccessToken(wrongCfg, token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestJWT_WrongIssuer(t *testing.T) {
	cfg := testJWTConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-123", "member", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	wrongCfg := testJWTConfig()
	wrongCfg.AppName = "other-app"

	_, err = auth.ValidateAccessToken(wrongCfg, token)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

func TestJWT_ClaimsFieldsPresent(t *testing.T) {
	cfg := testJWTConfig()
	perms := []string{"user:read"}

	token, err := auth.GenerateAccessToken(cfg, "user-456", "viewer", perms)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := auth.ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.ID == "" {
		t.Error("expected non-empty jti (ID)")
	}
	if claims.IssuedAt == nil {
		t.Error("expected non-nil IssuedAt")
	}
	if claims.ExpiresAt == nil {
		t.Error("expected non-nil ExpiresAt")
	}
	if len(claims.Audience) == 0 || !strings.Contains(claims.Audience[0], "gnha") {
		t.Errorf("expected gnha audience, got %v", claims.Audience)
	}
	if claims.Issuer != cfg.AppName {
		t.Errorf("expected issuer=%s, got %s", cfg.AppName, claims.Issuer)
	}
}
