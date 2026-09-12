package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"

	// pgx postgres driver for database/sql
	_ "github.com/jackc/pgx/v5/stdlib"

	stdsql "database/sql"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	m, err := newMigrate(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init migrate: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		err = m.Up()
	case "down":
		n := 1
		if len(os.Args) >= 3 {
			n, err = strconv.Atoi(os.Args[2])
			if err != nil {
				fmt.Fprintln(os.Stderr, "invalid step count")
				os.Exit(1)
			}
		}
		err = m.Steps(-n)
	case "version":
		printVersion(m)
		return
	case "force":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: migrate force <version>")
			os.Exit(1)
		}
		v, convErr := strconv.Atoi(os.Args[2])
		if convErr != nil {
			fmt.Fprintln(os.Stderr, "invalid version")
			os.Exit(1)
		}
		// golang-migrate accepts -1 to mean "no migration applied"; anything
		// below that is meaningless and Force would reject it anyway.
		if v < -1 {
			fmt.Fprintln(os.Stderr, "invalid version: must be >= -1")
			os.Exit(1)
		}
		printVersion(m)
		err = m.Force(v)
		if err == nil {
			fmt.Fprintf(os.Stderr, "forced schema_migrations to version=%d dirty=false\n", v)
			fmt.Fprintln(os.Stderr, "NOTE: this only changes the recorded version. It does not undo or")
			fmt.Fprintln(os.Stderr, "      apply any SQL — the schema is whatever the failed migration left")
			fmt.Fprintln(os.Stderr, "      behind. Verify it before running `up`. See docs/RUNBOOKS.md §8.")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintf(os.Stderr, "migrate error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("done")
}

func usage() {
	fmt.Fprintln(os.Stderr, strings.TrimSpace(`
usage: migrate <command>

  up                 apply all pending migrations
  down [N]           roll back N migrations (default 1)
  version            print the recorded version and dirty flag
  force <version>    set the recorded version and clear the dirty flag

"force" is the recovery path for a dirty migration: when a migration fails
part-way, golang-migrate records dirty=true and every later up/down refuses to
run, so the app cannot start. It does NOT run or undo SQL -- it only rewrites
what schema_migrations says. Inspect the schema first and use the lowest version
you know the database actually matches. docs/RUNBOOKS.md section 8 has the
procedure for the known wedge at migration 22.`))
}

// printVersion reports the recorded version, or says so when no migration has
// been applied. It never fails the process: it is diagnostic output, used both
// standalone and to record the before-state of a force.
func printVersion(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		fmt.Fprintln(os.Stderr, "current: no migration applied")
	case err != nil:
		fmt.Fprintf(os.Stderr, "current: unknown (%v)\n", err)
	default:
		fmt.Fprintf(os.Stderr, "current: version=%d dirty=%t\n", v, dirty)
	}
}

func newMigrate(dbURL string) (*migrate.Migrate, error) {
	db, err := stdsql.Open("pgx", dbURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("postgres driver: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}

	return migrate.NewWithInstance("iofs", src, "postgres", driver)
}
