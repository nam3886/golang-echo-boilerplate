package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/retry"
)

func TestConnect_SuccessOnFirstTry(t *testing.T) {
	calls := 0
	result, err := retry.Connect(context.Background(), "test", 3, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected %q, got %q", "ok", result)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestConnect_SuccessAfterRetries(t *testing.T) {
	calls := 0
	result, err := retry.Connect(context.Background(), "test", 5, func() (int, error) {
		calls++
		if calls < 3 {
			return 0, errors.New("not ready")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestConnect_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := retry.Connect(ctx, "test", 10, func() (string, error) {
		return "", errors.New("always fails")
	})
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestConnect_MaxRetriesExceeded(t *testing.T) {
	calls := 0
	sentinel := errors.New("permanent failure")

	_, err := retry.Connect(context.Background(), "test", 3, func() (string, error) {
		calls++
		return "", sentinel
	})
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error in chain, got: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected exactly 3 calls, got %d", calls)
	}
}
