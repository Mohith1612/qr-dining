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
	case "pending":
		os.Exit(pending(m))
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
  pending            report migrations this BINARY carries that the database
                     has not applied, as key=value lines on stdout
  force <version>    set the recorded version and clear the dirty flag

"pending" is the deploy pre-flight. It answers "would starting this image
change the schema?" and never writes: exit 0 = nothing pending, 10 = pending,
11 = the recorded version is dirty, 12 = the database is AHEAD of this binary.
See deploy/vm/deploy.sh and docs/OPERATIONS.md, "Automatic deployment".

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

// Exit codes for "pending". They are a deploy-time API: deploy/vm/deploy.sh
// branches on them, so they must not be renumbered.
const (
	exitPendingNone  = 0  // the database has already applied everything this binary carries
	exitPendingSome  = 10 // starting this image WOULD change the schema
	exitPendingDirty = 11 // schema_migrations.dirty is true; a previous migration failed part-way
	exitPendingAhead = 12 // the database is at a HIGHER version than this binary knows about
)

// pending reports what starting this image would do to the schema, and writes
// nothing. It is the deploy pre-flight.
//
// It deliberately reads the SAME embedded filesystem that cmd/server applies at
// boot (migrations.FS) rather than a migrations/ directory next to it. A
// directory can be stale, can belong to a different commit, or can be absent
// entirely — the runtime image has no source tree. The only honest answer to
// "would this image migrate the database?" comes from the image.
//
// Output is key=value lines on stdout so a shell can read it without parsing
// prose; the human summary goes to stderr.
func pending(m *migrate.Migrate) int {
	versions, err := sourceVersions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read embedded migrations: %v\n", err)
		return 1
	}
	if len(versions) == 0 {
		fmt.Fprintln(os.Stderr, "this binary embeds no migrations — refusing to guess")
		return 1
	}
	highest := versions[len(versions)-1]

	dbVersion, dirty, err := m.Version()
	applied := true
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		// No row in schema_migrations: an empty database. Everything is pending.
		applied = false
		dbVersion, dirty = 0, false
	case err != nil:
		fmt.Fprintf(os.Stderr, "read schema_migrations: %v\n", err)
		return 1
	}

	var waiting []uint
	for _, v := range versions {
		if !applied || v > dbVersion {
			waiting = append(waiting, v)
		}
	}

	fmt.Printf("db_applied=%t\n", applied)
	fmt.Printf("db_version=%d\n", dbVersion)
	fmt.Printf("db_dirty=%t\n", dirty)
	fmt.Printf("image_highest=%d\n", highest)
	fmt.Printf("image_count=%d\n", len(versions))
	fmt.Printf("pending_count=%d\n", len(waiting))
	fmt.Printf("pending=%s\n", joinVersions(waiting))

	switch {
	case dirty:
		fmt.Fprintf(os.Stderr, "DIRTY: schema_migrations records version=%d dirty=true. A migration\n", dbVersion)
		fmt.Fprintln(os.Stderr, "failed part-way and the schema is whatever it left behind. Do not deploy.")
		fmt.Fprintln(os.Stderr, "docs/RECOVERY.md has the force/inspect procedure.")
		return exitPendingDirty
	case applied && dbVersion > highest:
		// Deploying an older image onto a newer schema. The binary rollback is
		// the only safe lever we have, so this must be visible, not inferred.
		fmt.Fprintf(os.Stderr, "AHEAD: database is at version=%d but this image only knows %d.\n", dbVersion, highest)
		fmt.Fprintln(os.Stderr, "The schema is forward of the binary. Nothing here can roll a schema back;")
		fmt.Fprintln(os.Stderr, "verify this older binary tolerates the newer schema before proceeding.")
		return exitPendingAhead
	case len(waiting) > 0:
		fmt.Fprintf(os.Stderr, "PENDING: %d migration(s) would be applied at boot: %s\n",
			len(waiting), joinVersions(waiting))
		return exitPendingSome
	default:
		fmt.Fprintf(os.Stderr, "UP TO DATE: database at version=%d, image highest=%d.\n", dbVersion, highest)
		return exitPendingNone
	}
}

// sourceVersions lists every migration version embedded in this binary, ascending.
func sourceVersions() ([]uint, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}
	defer func() { _ = src.Close() }()

	v, err := src.First()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("first migration: %w", err)
	}

	out := []uint{v}
	for {
		next, nextErr := src.Next(v)
		if errors.Is(nextErr, os.ErrNotExist) {
			return out, nil
		}
		if nextErr != nil {
			return nil, fmt.Errorf("migration after %d: %w", v, nextErr)
		}
		out = append(out, next)
		v = next
	}
}

func joinVersions(vs []uint) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(parts, ",")
}
