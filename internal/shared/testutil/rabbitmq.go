package testutil

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

// NewTestRabbitMQ starts a temporary RabbitMQ container and returns an AMQP connection URL.
// The container is cleaned up automatically via t.Cleanup.
func NewTestRabbitMQ(t *testing.T) string {
	t.Helper()
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
