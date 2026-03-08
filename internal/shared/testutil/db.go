package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestPostgres returns a pgxpool.Pool for integration tests.
// If DATABASE_URL is set (e.g. GitLab CI service containers), it connects directly.
// Otherwise, it starts a local testcontainer for development.
func NewTestPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			t.Fatalf("connecting to DATABASE_URL: %v", err)
		}
		t.Cleanup(func() { pool.Close() })
		return pool
	}

	// Fallback: start a testcontainer for local dev
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pool
}
