// Package testutil provides helpers for integration tests that require a real database.
// All helpers skip the test if TEST_DATABASE_URL is not set.
package testutil

import (
	"context"
	"os"
	"testing"

	dbPkg "github.com/Mohith1612/qr-dining/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenTestDB connects to TEST_DATABASE_URL, runs migrations, and returns the pool.
// The pool is closed automatically via t.Cleanup.
// Skips the test if TEST_DATABASE_URL is not set.
func OpenTestDB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := dbPkg.RunMigrations(pool); err != nil {
		pool.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// TruncateTables truncates the given tables with CASCADE. Call in t.Cleanup
// to isolate tests from each other.
func TruncateTables(t testing.TB, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	ctx := context.Background()
	for _, table := range tables {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
