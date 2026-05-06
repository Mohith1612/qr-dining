package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) GetTableByQRToken(ctx context.Context, token string) (sqlc.Table, error) {
	t, err := r.q.GetTableByQRToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Table{}, domain.ErrTableNotFound
	}
	return t, err
}

func (r *Repos) GetTableByID(ctx context.Context, id int64) (sqlc.Table, error) {
	t, err := r.q.GetTableByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Table{}, domain.ErrTableNotFound
	}
	return t, err
}

func (r *Repos) UpdateTableStatus(ctx context.Context, id int64, status sqlc.TableStatus) error {
	return r.q.UpdateTableStatus(ctx, sqlc.UpdateTableStatusParams{
		ID:     id,
		Status: status,
	})
}

func (r *Repos) ListTablesForBranch(ctx context.Context, branchID int64) ([]sqlc.Table, error) {
	return r.q.ListTablesForBranch(ctx, branchID)
}
