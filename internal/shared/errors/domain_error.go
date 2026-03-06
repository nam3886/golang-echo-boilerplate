package errors

import "net/http"

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

// HTTPStatus maps error code to HTTP status.
func (e *DomainError) HTTPStatus() int {
	if status, ok := codeToHTTP[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Sentinel errors for common cases.
var (
	ErrNotFound      = &DomainError{Code: CodeNotFound, Message: "not found"}
	ErrAlreadyExists = &DomainError{Code: CodeAlreadyExists, Message: "already exists"}
	ErrForbidden     = &DomainError{Code: CodePermissionDenied, Message: "forbidden"}
	ErrUnauthorized  = &DomainError{Code: CodeUnauthenticated, Message: "unauthorized"}
	ErrInternal      = &DomainError{Code: CodeInternal, Message: "internal error"}
)

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
