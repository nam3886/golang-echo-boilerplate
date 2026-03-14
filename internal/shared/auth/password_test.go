package auth_test

import (
	"strings"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
)

func TestPassword_RoundTrip(t *testing.T) {
	h := auth.NewPasswordHasher()

	encoded, err := h.Hash("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	ok, err := h.Verify("correct-horse-battery", encoded)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Error("expected Verify to return true for correct password")
	}
}

func TestPassword_WrongPassword(t *testing.T) {
	h := auth.NewPasswordHasher()

	encoded, err := h.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	ok, err := h.Verify("wrong-password", encoded)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Error("expected Verify to return false for wrong password")
	}
}

func TestPassword_TooShort(t *testing.T) {
	h := auth.NewPasswordHasher()

	_, err := h.Hash("short")
	if err == nil {
		t.Fatal("expected error for password shorter than 8 bytes")
	}
}

func TestPassword_TooLong(t *testing.T) {
	h := auth.NewPasswordHasher()

	longPwd := strings.Repeat("a", 73)
	_, err := h.Hash(longPwd)
	if err == nil {
		t.Fatal("expected error for password exceeding 72 bytes")
	}
}

func TestPassword_HashFormat(t *testing.T) {
	h := auth.NewPasswordHasher()

	encoded, err := h.Hash("valid-password-123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Errorf("expected hash to start with $argon2id$, got %q", encoded[:10])
	}
}

func TestPassword_VerifyOversized_ReturnsFalse(t *testing.T) {
	h := auth.NewPasswordHasher()
	oversized := strings.Repeat("a", 100) // exceeds argon2id 72-byte limit
	// Early-return for oversized input never reaches hash parsing, so any valid argon2id hash format works.
	ok, err := h.Verify(oversized, "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ok {
		t.Error("expected false for oversized password")
	}
}
