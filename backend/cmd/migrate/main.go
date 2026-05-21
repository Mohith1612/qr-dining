package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

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
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down [N]>")
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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintf(os.Stderr, "migrate error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("done")
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
