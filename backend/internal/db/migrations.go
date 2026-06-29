package db

import (
	"errors"
	"fmt"

	"github.com/Mohith1612/qr-dining/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations applies all pending up migrations from the embedded SQL files.
// Called once at startup, before the HTTP server accepts traffic.
// The app always migrates itself — never rely on an external CLI in production.
func RunMigrations(pool *pgxpool.Pool) error {
	// Open a standard database/sql connection from the pgxpool for golang-migrate.
	// Close it when done so its borrowed connections return to the pool — this is
	// a no-op for the long-lived app pool, but it lets callers that close the pool
	// (e.g. tests) shut down cleanly instead of blocking on leaked connections.
	db := pgxstdlib.OpenDBFromPool(pool)
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migrate driver: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create iofs migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	// Release the dedicated connection golang-migrate pins for the migration
	// advisory lock back to the pool when done. Without this, a caller that
	// closes the pool (e.g. tests) blocks forever on the leaked connection.
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
