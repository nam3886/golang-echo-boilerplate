package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	// Navigate from internal/shared/testutil/ up to project root
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	dir := filepath.Join(root, "db", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir not found at %s: %v", dir, err)
	}
	return dir
}
