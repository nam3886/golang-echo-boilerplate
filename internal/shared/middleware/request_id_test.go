package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequestID_Missing_GeneratesUUID(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	rid := rec.Header().Get(echo.HeaderXRequestID)
	if rid == "" {
		t.Error("expected request ID to be generated")
	}
}

func TestRequestID_Valid_Preserved(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, "my-valid-id-123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get(echo.HeaderXRequestID) != "my-valid-id-123" {
		t.Error("expected valid request ID to be preserved")
	}
}

func TestRequestID_InvalidChars_Regenerated(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, "bad\r\nid")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	rid := rec.Header().Get(echo.HeaderXRequestID)
	if rid == "bad\r\nid" {
		t.Error("expected invalid request ID to be replaced")
	}
	if rid == "" {
		t.Error("expected a new request ID to be generated")
	}
}

func TestRequestID_TooLong_Regenerated(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, strings.Repeat("x", 200))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	rid := rec.Header().Get(echo.HeaderXRequestID)
	if len(rid) > 128 {
		t.Errorf("expected regenerated ID <= 128 chars, got %d", len(rid))
	}
}

func TestRequestID_Empty_GeneratesUUID(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	rid := rec.Header().Get(echo.HeaderXRequestID)
	if rid == "" {
		t.Error("expected request ID to be generated for empty header")
	}
}
