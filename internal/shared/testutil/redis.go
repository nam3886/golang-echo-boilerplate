package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// NewTestRedis starts a temporary Redis container and returns a connected *redis.Client.
// In CI, set REDIS_URL to use the service container instead of testcontainers.
// The container and client are cleaned up automatically via t.Cleanup.
func NewTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	// In CI, use the service container instead of testcontainers.
	if url := os.Getenv("REDIS_URL"); url != "" {
		opt, err := redis.ParseURL(url)
		if err != nil {
			t.Fatalf("parsing REDIS_URL: %v", err)
		}
		client := redis.NewClient(opt)
		t.Cleanup(func() { _ = client.Close() })
		return client
	}

	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("starting redis container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("getting redis connection string: %v", err)
	}

	opt, err := redis.ParseURL(connStr)
	if err != nil {
		t.Fatalf("parsing redis URL: %v", err)
	}

	client := redis.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	return client
}
