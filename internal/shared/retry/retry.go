// Package retry provides shared connection retry utilities.
package retry

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Connect retries a connection function with exponential backoff.
// Returns the first successful result or an error after maxRetries.
// The context is checked between retries to support cancellation.
func Connect[T any](ctx context.Context, name string, maxRetries int, fn func() (T, error)) (T, error) {
	var (
		result T
		err    error
	)
	for i := range maxRetries {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		slog.Warn(name+" not ready, retrying", "attempt", i+1, "err", err)
		timer := time.NewTimer(min(time.Duration(1<<uint(i))*time.Second, 30*time.Second))
		select {
		case <-ctx.Done():
			timer.Stop()
			var zero T
			return zero, fmt.Errorf("%s connection cancelled: %w", name, ctx.Err())
		case <-timer.C:
		}
	}
	var zero T
	return zero, fmt.Errorf("%s connection failed after %d retries: %w", name, maxRetries, err)
}

// Do retries a void operation with exponential backoff.
// Convenience wrapper around Connect for operations that return no value.
func Do(ctx context.Context, name string, maxRetries int, fn func() error) error {
	_, err := Connect(ctx, name, maxRetries, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}
