package testutil

import (
	"errors"
	"testing"

	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// AssertPanics verifies that fn panics. Reports failure with the given label.
func AssertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", label)
		}
	}()
	fn()
}

// AssertDomainError checks that err is a DomainError with the expected message.
// Unlike errors.Is, this checks identity (message), not just category (code).
// Use this when testing that a specific error message is returned, not just an error code.
//
// Example:
//
//	testutil.AssertDomainError(t, err, "email is already taken")
func AssertDomainError(t *testing.T, err error, wantMsg string) {
	t.Helper()
	var de *sharederr.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("expected DomainError, got %T: %v", err, err)
	}
	if de.Message != wantMsg {
		t.Errorf("DomainError message = %q, want %q", de.Message, wantMsg)
	}
}
