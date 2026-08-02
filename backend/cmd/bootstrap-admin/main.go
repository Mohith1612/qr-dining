package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const minBootstrapPasswordLen = 14

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap admin: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	email := services.NormalizePlatformEmail(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	if email == "" {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL is required")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL must be a plain valid email address")
	}

	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if len(password) < minBootstrapPasswordLen {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must be at least %d characters", minBootstrapPasswordLen)
	}
	displayName := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DISPLAY_NAME"))
	if displayName == "" {
		displayName = "Platform Super Admin"
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	queries := sqlc.New(tx)
	if _, err := queries.GetPlatformUserByEmail(ctx, email); err == nil {
		return fmt.Errorf("platform user %q already exists; refusing to reset it", email)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check existing platform user: %w", err)
	}

	passwordHash, err := services.HashPlatformPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user, err := queries.UpsertPlatformUser(ctx, sqlc.UpsertPlatformUserParams{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		Status:       "active",
		MfaRequired:  true,
	})
	if err != nil {
		return fmt.Errorf("create platform user: %w", err)
	}
	if err := queries.AddPlatformUserRole(ctx, sqlc.AddPlatformUserRoleParams{
		PlatformUserID: user.ID,
		Role:           services.PlatformRoleSuperAdmin,
	}); err != nil {
		return fmt.Errorf("grant super-admin role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("created platform super-admin %s; sign in and enroll TOTP immediately\n", email)
	return nil
}
