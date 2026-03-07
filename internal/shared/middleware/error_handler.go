package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
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

	var domErr *domainerr.DomainError
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
		_ = c.JSON(echoErr.Code, ErrorResponse{
			Code:    domainerr.CodeInternal.String(),
			Message: msg,
		})
		return
	}

	// Unexpected error — log and return generic 500
	slog.Error("unhandled error", "err", err, "path", c.Request().URL.Path)
	_ = c.JSON(http.StatusInternalServerError, ErrorResponse{
		Code:    string(domainerr.CodeInternal),
		Message: "internal error",
	})
}
