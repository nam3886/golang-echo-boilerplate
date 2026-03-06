package testutil

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// NewTestRedis starts a temporary Redis container and returns a connected *redis.Client.
// The container and client are cleaned up automatically via t.Cleanup.
func NewTestRedis(t *testing.T) *redis.Client {
	t.Helper()
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
