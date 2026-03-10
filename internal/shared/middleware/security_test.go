package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/labstack/echo/v4"
)

func TestSecurityHeaders_SetsAllHeaders(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeaders(&config.Config{AppEnv: "staging"})) // non-dev
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}
	for key, want := range headers {
		got := rec.Header().Get(key)
		if got != want {
			t.Errorf("header %s: want %q, got %q", key, want, got)
		}
	}

	// HSTS should be set in non-dev
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS header in non-dev mode")
	}
}

func TestSecurityHeaders_DevMode_NoHSTS(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeaders(&config.Config{AppEnv: "development"})) // dev mode
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set in dev mode")
	}
}
