package domainerr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeNotFound, "user not found")
	if err.Code != CodeNotFound {
		t.Errorf("expected code %s, got %s", CodeNotFound, err.Code)
	}
	if err.Message != "user not found" {
		t.Errorf("expected message 'user not found', got %s", err.Message)
	}
	if err.Err != nil {
		t.Errorf("expected nil underlying error, got %v", err.Err)
	}
}

func TestError(t *testing.T) {
	err := New(CodeNotFound, "not found")
	if err.Error() != "not found" {
		t.Errorf("expected 'not found', got %s", err.Error())
	}
}

func TestIs_MatchesByCode(t *testing.T) {
	err1 := New(CodeNotFound, "user not found")
	err2 := New(CodeNotFound, "resource not found")

	if !err1.Is(err2) {
		t.Error("Is should match by error code, not pointer identity")
	}
	if !err2.Is(err1) {
		t.Error("Is should be symmetric")
	}
}

func TestIs_DifferentCode(t *testing.T) {
	err1 := New(CodeNotFound, "not found")
	err2 := New(CodeAlreadyExists, "already exists")

	if err1.Is(err2) {
		t.Error("Is should return false for different error codes")
	}
}

func TestIs_NonDomainError(t *testing.T) {
	err1 := New(CodeNotFound, "not found")
	err2 := fmt.Errorf("some other error")

	if err1.Is(err2) {
		t.Error("Is should return false for non-DomainError targets")
	}
}

func TestWrap_PreservesUnderlying(t *testing.T) {
	underlying := fmt.Errorf("db connection lost")
	err := Wrap(CodeInternal, "database failure", underlying)

	if err.Err != underlying {
		t.Errorf("expected underlying error to be preserved")
	}
	if err.Unwrap() != underlying {
		t.Errorf("Unwrap should return the underlying error")
	}
}

func TestUnwrap_Nil(t *testing.T) {
	err := New(CodeNotFound, "not found")
	if err.Unwrap() != nil {
		t.Errorf("Unwrap should return nil for non-wrapped errors")
	}
}

func TestHTTPStatus_KnownCodes(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{CodeInvalidArgument, http.StatusBadRequest},
		{CodeNotFound, http.StatusNotFound},
		{CodeAlreadyExists, http.StatusConflict},
		{CodePermissionDenied, http.StatusForbidden},
		{CodeUnauthenticated, http.StatusUnauthorized},
		{CodeFailedPrecondition, http.StatusPreconditionFailed},
		{CodeInternal, http.StatusInternalServerError},
		{CodeUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := New(tt.code, "test")
			if status := err.HTTPStatus(); status != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, status)
			}
		})
	}
}

func TestHTTPStatus_UnknownCode(t *testing.T) {
	err := &DomainError{Code: ErrorCode("UNKNOWN"), Message: "test"}
	if status := err.HTTPStatus(); status != http.StatusInternalServerError {
		t.Errorf("expected 500 for unknown code, got %d", status)
	}
}

func TestErrNotFound_FreshPointer(t *testing.T) {
	err1 := ErrNotFound()
	err2 := ErrNotFound()

	if err1 == err2 {
		t.Error("ErrNotFound should return fresh pointers, not shared state")
	}
	if !err1.Is(err2) {
		t.Error("but pointers should match by error code")
	}
}

func TestErrorsIs_WithWrapping(t *testing.T) {
	underlying := fmt.Errorf("db error")
	err := Wrap(CodeNotFound, "user not found", underlying)

	if !errors.Is(err, ErrNotFound()) {
		t.Error("errors.Is should work through wrapping via custom Is method")
	}
}
