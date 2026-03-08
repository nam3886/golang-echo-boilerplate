// Package domainerr defines domain-level error types and sentinel constructors.
package domainerr

import (
	"errors"
	"net/http"
)

// ErrorCode represents a domain error classification.
type ErrorCode string

const (
	CodeInvalidArgument    ErrorCode = "INVALID_ARGUMENT"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeAlreadyExists      ErrorCode = "ALREADY_EXISTS"
	CodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
	CodeUnauthenticated    ErrorCode = "UNAUTHENTICATED"
	CodeFailedPrecondition ErrorCode = "FAILED_PRECONDITION"
	CodeInternal           ErrorCode = "INTERNAL"
	CodeUnavailable        ErrorCode = "UNAVAILABLE"
)

// DomainError is the base error type for all domain-level errors.
type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (c ErrorCode) String() string   { return string(c) }
func (e *DomainError) Error() string { return e.Message }
func (e *DomainError) Unwrap() error { return e.Err }

// Is matches by error category (ErrorCode), not by specific error identity.
// This means errors.Is(ErrUserNotFound(), ErrOrderNotFound()) returns true
// because both have CodeNotFound. This is intentional for HTTP status mapping.
// For identity-specific matching, use errors.As and check the Message field.
func (e *DomainError) Is(target error) bool {
	var t *DomainError
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// HTTPStatus maps error code to HTTP status.
func (e *DomainError) HTTPStatus() int {
	if status, ok := codeToHTTP[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Sentinel error templates — unexported to prevent external mutation.
var (
	errNotFound      = DomainError{Code: CodeNotFound, Message: "not found"}
	errAlreadyExists = DomainError{Code: CodeAlreadyExists, Message: "already exists"}
	errForbidden     = DomainError{Code: CodePermissionDenied, Message: "forbidden"}
	errUnauthorized  = DomainError{Code: CodeUnauthenticated, Message: "unauthorized"}
	errInternal      = DomainError{Code: CodeInternal, Message: "internal error"}
)

// ErrNotFound returns a fresh not-found sentinel error.
func ErrNotFound() *DomainError { e := errNotFound; return &e }

// ErrAlreadyExists returns a fresh already-exists sentinel error.
func ErrAlreadyExists() *DomainError { e := errAlreadyExists; return &e }

// ErrForbidden returns a fresh forbidden sentinel error.
func ErrForbidden() *DomainError { e := errForbidden; return &e }

// ErrUnauthorized returns a fresh unauthorized sentinel error.
func ErrUnauthorized() *DomainError { e := errUnauthorized; return &e }

// ErrInternal returns a fresh internal sentinel error.
func ErrInternal() *DomainError { e := errInternal; return &e }

// New creates a new DomainError with given code and message.
func New(code ErrorCode, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}

// Wrap creates a new DomainError wrapping an underlying error.
func Wrap(code ErrorCode, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err}
}

var codeToHTTP = map[ErrorCode]int{
	CodeInvalidArgument:    http.StatusBadRequest,
	CodeNotFound:           http.StatusNotFound,
	CodeAlreadyExists:      http.StatusConflict,
	CodePermissionDenied:   http.StatusForbidden,
	CodeUnauthenticated:    http.StatusUnauthorized,
	CodeFailedPrecondition: http.StatusPreconditionFailed,
	CodeInternal:           http.StatusInternalServerError,
	CodeUnavailable:        http.StatusServiceUnavailable,
}
