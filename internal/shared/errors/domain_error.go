// Package sharederr defines domain-level error types and sentinel constructors.
package sharederr

import (
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
	CodeResourceExhausted  ErrorCode = "RESOURCE_EXHAUSTED"
	CodeInternal           ErrorCode = "INTERNAL"
	CodeUnavailable        ErrorCode = "UNAVAILABLE"
)

// DomainError is the base error type for all domain-level errors.
type DomainError struct {
	Code    ErrorCode `json:"code"`
	Key     string    `json:"key,omitempty"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (c ErrorCode) String() string   { return string(c) }
func (e *DomainError) Error() string { return e.Message }
func (e *DomainError) Unwrap() error { return e.Err }

// Is matches by Code+Key when both errors have a Key set, otherwise by Code alone.
// Key-based matching enables precise sentinel matching (e.g. errors.Is(err, ErrNotFound()))
// while Code-only matching remains available for HTTP status mapping when Key is absent.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	if e.Key != "" && t.Key != "" {
		return e.Code == t.Code && e.Key == t.Key
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
	errNotFound      = DomainError{Code: CodeNotFound, Key: "not_found", Message: "not found"}
	errAlreadyExists = DomainError{Code: CodeAlreadyExists, Key: "already_exists", Message: "already exists"}
	errForbidden     = DomainError{Code: CodePermissionDenied, Key: "forbidden", Message: "forbidden"}
	errUnauthorized  = DomainError{Code: CodeUnauthenticated, Key: "unauthorized", Message: "unauthorized"}
	errInternal      = DomainError{Code: CodeInternal, Key: "internal", Message: "internal error"}
	errNoChange      = DomainError{Code: CodeFailedPrecondition, Key: "no_change", Message: "no change"}
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

// ErrNoChange signals that an update closure made no mutations.
// The repository should skip the SQL UPDATE when it receives this.
func ErrNoChange() *DomainError { e := errNoChange; return &e }

// New creates a new DomainError with given code, key, and message.
func New(code ErrorCode, key, message string) *DomainError {
	return &DomainError{Code: code, Key: key, Message: message}
}

// Wrap creates a new DomainError wrapping an underlying error.
func Wrap(code ErrorCode, key, message string, err error) *DomainError {
	return &DomainError{Code: code, Key: key, Message: message, Err: err}
}

var codeToHTTP = map[ErrorCode]int{
	CodeInvalidArgument:    http.StatusBadRequest,
	CodeNotFound:           http.StatusNotFound,
	CodeAlreadyExists:      http.StatusConflict,
	CodePermissionDenied:   http.StatusForbidden,
	CodeUnauthenticated:    http.StatusUnauthorized,
	CodeFailedPrecondition: http.StatusPreconditionFailed,
	CodeResourceExhausted:  http.StatusTooManyRequests,
	CodeInternal:           http.StatusInternalServerError,
	CodeUnavailable:        http.StatusServiceUnavailable,
}
