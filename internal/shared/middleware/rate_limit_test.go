package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newRateLimitEcho(rdb *redis.Client, limit int, window time.Duration) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/", okHandler, RateLimit(rdb, limit, window))
	return e
}

// Note: slidingWindowCount adds the current request THEN compares count > limit.
// So with limit=N, requests 1..N succeed; the (N+1)th request is the first to be rejected.

func TestRateLimit_UnderLimit_Passes(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	const limit = 5
	e := newRateLimitEcho(rdb, limit, time.Minute)

	// limit-1 requests should all pass (count after add = 1..4, all < 5).
	for i := 0; i < limit-1; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_OverLimit_Returns429(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	const limit = 3
	e := newRateLimitEcho(rdb, limit, time.Minute)

	// First limit requests pass (count 1..limit, none > limit).
	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// The (limit+1)th request hits count > limit → rejected with 429.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on request %d, got %d", limit+1, rec.Code)
	}
}

func TestRateLimit_RedisFailure_FailsOpen(t *testing.T) {
	// Nothing listening on this port — simulates Redis outage.
	// The limiter must fail open (allow the request) so a Redis blip
	// does not take down the entire service.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:19998"})

	e := newRateLimitEcho(rdb, 10, time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 on redis failure (fail-open), got %d", rec.Code)
	}
}

// Rate limiter is always IP-based; auth context does not change the key.
func TestRateLimit_WithAuthContext_StillKeyedByIP(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// limit=3 → 3 requests pass, 4th is rejected (count > limit).
	const limit = 3
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/", okHandler,
		injectClaims("u1", "member", nil),
		RateLimit(rdb, limit, time.Minute),
	)

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Next request should be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for user rate limit, got %d", rec.Code)
	}
}
