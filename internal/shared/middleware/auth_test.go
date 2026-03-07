package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/config"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// testConfig returns a minimal Config suitable for JWT tests.
func testConfig() *config.Config {
	return &config.Config{
		AppName:      "test-service",
		JWTSecret:    "test-secret-must-be-at-least-32-chars!!",
		JWTAccessTTL: 15 * time.Minute,
	}
}

// newEchoWithAuth wires the Auth middleware onto a simple "ok" handler.
func newEchoWithAuth(cfg *config.Config, rdb *redis.Client) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, Auth(cfg, rdb))
	return e
}

func TestAuth_ValidToken_Passes(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-1", "admin", []string{"user:read"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_MissingHeader_Returns401(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_InvalidToken_Returns401(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ExpiredToken_Returns401(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	// Generate a token that is already expired.
	expiredCfg := &config.Config{
		AppName:      cfg.AppName,
		JWTSecret:    cfg.JWTSecret,
		JWTAccessTTL: -1 * time.Second, // expires in the past
	}
	token, err := auth.GenerateAccessToken(expiredCfg, "user-1", "member", nil)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_BlacklistedToken_Returns401(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-1", "admin", nil)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	// Parse the token to get the JTI so we can blacklist it.
	claims, err := auth.ValidateAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	jti := claims.ID
	rdb.Set(context.Background(), "blacklist:"+jti, "1", time.Minute)

	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_RedisFailure_Returns401(t *testing.T) {
	// Point Redis at an address with nothing listening — simulates failure.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:19999"})

	cfg := testConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-1", "admin", nil)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	e := newEchoWithAuth(cfg, rdb)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on redis failure (fail-closed), got %d", rec.Code)
	}
}

func TestAuth_SetsUserInContext(t *testing.T) {
	mr, _ := miniredis.Run()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	cfg := testConfig()
	token, err := auth.GenerateAccessToken(cfg, "user-42", "admin", []string{"user:read"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	var capturedUserID string
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/", func(c echo.Context) error {
		u := auth.UserFromContext(c.Request().Context())
		if u != nil {
			capturedUserID = u.UserID
		}
		return c.String(http.StatusOK, "ok")
	}, Auth(cfg, rdb))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUserID != "user-42" {
		t.Errorf("expected user-42 in context, got %q", capturedUserID)
	}
}

// ensure the domainerr package is used (avoids unused import if only referenced via ErrorHandler).
var _ = domainerr.ErrUnauthorized
