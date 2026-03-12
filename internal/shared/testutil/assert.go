package testutil

import (
	"errors"
	"testing"

	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// AssertDomainError checks that err is a DomainError with the expected message.
// Unlike errors.Is, this checks identity (message), not just category (code).
// Use this when testing that a specific error message is returned, not just an error code.
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
