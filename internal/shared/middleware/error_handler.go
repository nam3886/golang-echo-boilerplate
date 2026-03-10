package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/labstack/echo/v4"
)

// ErrorResponse is the standard JSON error response body.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler is a centralized Echo error handler that translates
// DomainError into proper HTTP responses.
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var domErr *sharederr.DomainError
	if errors.As(err, &domErr) {
		_ = c.JSON(domErr.HTTPStatus(), ErrorResponse{
			Code:    string(domErr.Code),
			Message: domErr.Message,
		})
		return
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		msg := "error"
		if m, ok := echoErr.Message.(string); ok {
			msg = m
		}
		code := echoHTTPToDomainCode(echoErr.Code)
		_ = c.JSON(echoErr.Code, ErrorResponse{
			Code:    code.String(),
			Message: msg,
		})
		return
	}

	// Unexpected error — log and return generic 500
	slog.Error("unhandled error", "err", err, "path", c.Request().URL.Path)
	_ = c.JSON(http.StatusInternalServerError, ErrorResponse{
		Code:    sharederr.CodeInternal.String(),
		Message: "internal error",
	})
}

// echoHTTPToDomainCode maps Echo HTTP status codes to domain error codes.
func echoHTTPToDomainCode(status int) sharederr.ErrorCode {
	switch status {
	case http.StatusNotFound:
		return sharederr.CodeNotFound
	case http.StatusMethodNotAllowed, http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return sharederr.CodeInvalidArgument
	case http.StatusTooManyRequests:
		return sharederr.CodeResourceExhausted
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return sharederr.CodeUnavailable
	default:
		return sharederr.CodeInternal
	}
}
