package repository

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Repos bundles all repositories. Services receive a *Repos and call methods on it.
// For transactions, call WithTx — it runs the callback with a tx-backed *Repos.
type Repos struct {
	pool   *pgxpool.Pool
	db     sqlc.DBTX // pool or tx — used for ExecRaw
	q      *sqlc.Queries
	logger zerolog.Logger
}

func New(pool *pgxpool.Pool, logger zerolog.Logger) *Repos {
	return &Repos{pool: pool, db: pool, q: sqlc.New(pool), logger: logger}
}

// WithTx executes fn inside a single database transaction. If fn returns an error,
// the transaction is rolled back. Otherwise it is committed.
func (r *Repos) WithTx(ctx context.Context, fn func(*Repos) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txRepos := &Repos{pool: r.pool, db: tx, q: sqlc.New(tx), logger: r.logger}
	if err := fn(txRepos); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ExecRaw executes a raw SQL statement on the current connection (pool or tx).
// Used for session-level statements like SET CONSTRAINTS.
func (r *Repos) ExecRaw(ctx context.Context, sql string, args ...any) error {
	_, err := r.db.Exec(ctx, sql, args...)
	return err
}
