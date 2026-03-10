package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/labstack/echo/v4"
)

func newTestEchoContext(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestErrorHandler_DomainError_NotFound(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(sharederr.ErrNotFound(), c)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestErrorHandler_DomainError_Unauthorized(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(sharederr.ErrUnauthorized(), c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestErrorHandler_DomainError_Forbidden(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(sharederr.ErrForbidden(), c)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestErrorHandler_DomainError_AlreadyExists(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(sharederr.ErrAlreadyExists(), c)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestErrorHandler_DomainError_Internal(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(sharederr.ErrInternal(), c)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestErrorHandler_EchoHTTPError(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(echo.NewHTTPError(http.StatusBadRequest, "bad input"), c)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestErrorHandler_UnknownError_Returns500(t *testing.T) {
	c, rec := newTestEchoContext(t)
	ErrorHandler(errors.New("something unexpected"), c)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestErrorHandler_CommittedResponse_Skipped(t *testing.T) {
	c, rec := newTestEchoContext(t)
	// Mark response as already committed — ErrorHandler must not write again.
	c.Response().Committed = true
	ErrorHandler(sharederr.ErrNotFound(), c)
	// No write should have happened: default recorder status is 200.
	if rec.Code != http.StatusOK {
		t.Errorf("expected no write (200 default), got %d", rec.Code)
	}
}
