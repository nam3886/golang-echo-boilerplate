package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

// NewTestRabbitMQ returns an AMQP connection URL for integration tests.
// In CI, set RABBITMQ_URL to use the service container instead of testcontainers.
// The container is cleaned up automatically via t.Cleanup.
func NewTestRabbitMQ(t *testing.T) string {
	t.Helper()

	// In CI, use the service container instead of testcontainers.
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		return url
	}

	ctx := context.Background()

	container, err := rabbitmq.Run(ctx, "rabbitmq:3-management-alpine")
	if err != nil {
		t.Fatalf("starting rabbitmq container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	url, err := container.AmqpURL(ctx)
	if err != nil {
		t.Fatalf("getting rabbitmq AMQP URL: %v", err)
	}

	return url
}
