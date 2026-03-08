// Package retry provides shared connection retry utilities.
package retry

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Connect retries a connection function with linear backoff.
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
		select {
		case <-ctx.Done():
			var zero T
			return zero, fmt.Errorf("%s connection cancelled: %w", name, ctx.Err())
		case <-time.After(time.Duration(i+1) * time.Second):
		}
	}
	var zero T
	return zero, fmt.Errorf("%s connection failed after %d retries: %w", name, maxRetries, err)
}
