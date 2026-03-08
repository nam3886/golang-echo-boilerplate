package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequestLogger_ErrorPropagation(t *testing.T) {
	e := echo.New()
	testErr := fmt.Errorf("handler error")

	// Handler that returns an error
	handler := func(_ echo.Context) error {
		return testErr
	}

	mw := RequestLogger()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call the wrapped handler
	err := wrappedHandler(c)

	// Regression test: error must be propagated (not nil)
	if err != testErr {
		t.Errorf("expected error %v to be propagated, got %v", testErr, err)
	}
}

func TestRequestLogger_SuccessReturnsNil(t *testing.T) {
	e := echo.New()

	// Handler that succeeds
	handler := func(_ echo.Context) error {
		return nil
	}

	mw := RequestLogger()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := wrappedHandler(c)

	if err != nil {
		t.Errorf("expected nil for successful handler, got %v", err)
	}
}

func TestRequestLogger_LogsRequestDetails(t *testing.T) {
	e := echo.New()

	handler := func(c echo.Context) error {
		c.Response().WriteHeader(http.StatusOK)
		if _, err := c.Response().Write([]byte("test")); err != nil {
			return err
		}
		return nil
	}

	mw := RequestLogger()
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Header.Set("User-Agent", "test-client")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := wrappedHandler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response was written
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "test" {
		t.Errorf("expected body 'test', got %s", rec.Body.String())
	}
}
