package testutil

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes all goose Up migrations from db/migrations/ on the test DB.
func RunMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	migrationsDir := findMigrationsDir(t)
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading migrations dir: %v", err)
	}

	// Sort by filename to ensure order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading migration %s: %v", entry.Name(), err)
		}
		// Extract only the Up portion (between "-- +goose Up" and "-- +goose Down")
		sql := extractGooseUp(string(data))
		if sql == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("executing migration %s: %v", entry.Name(), err)
		}
	}
}

// extractGooseUp returns SQL between "-- +goose Up" and "-- +goose Down".
func extractGooseUp(content string) string {
	upIdx := strings.Index(content, "-- +goose Up")
	if upIdx == -1 {
		return ""
	}
	sql := content[upIdx+len("-- +goose Up"):]
	downIdx := strings.Index(sql, "-- +goose Down")
	if downIdx != -1 {
		sql = sql[:downIdx]
	}
	return strings.TrimSpace(sql)
}

// findMigrationsDir locates db/migrations/ relative to the project root.
// It checks MIGRATIONS_DIR env var first (useful with -trimpath builds),
// then falls back to the current working directory (project root when running `go test ./...`).
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	if envDir := os.Getenv("MIGRATIONS_DIR"); envDir != "" {
		if _, err := os.Stat(envDir); err != nil {
			t.Fatalf("MIGRATIONS_DIR %q not found: %v", envDir, err)
		}
		return envDir
	}

	// Fallback: walk up from cwd until we find db/migrations.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	dir := filepath.Join(cwd, "db", "migrations")
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		cwd = filepath.Dir(cwd)
		dir = filepath.Join(cwd, "db", "migrations")
	}
	t.Fatalf("migrations dir not found (set MIGRATIONS_DIR env var or run tests from project root)")
	return ""
}
