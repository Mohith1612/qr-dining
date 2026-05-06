package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) GetStaffByID(ctx context.Context, id int64) (sqlc.Staff, error) {
	s, err := r.q.GetStaffByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Staff{}, domain.ErrInternalError
	}
	return s, err
}

func (r *Repos) ListStaffForBranch(ctx context.Context, branchID int64) ([]sqlc.Staff, error) {
	return r.q.ListStaffForBranch(ctx, branchID)
}

func (r *Repos) CreateStaff(ctx context.Context, p sqlc.CreateStaffParams) (sqlc.Staff, error) {
	return r.q.CreateStaff(ctx, p)
}
