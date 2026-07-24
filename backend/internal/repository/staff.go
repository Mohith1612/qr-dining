package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) GetStaffByID(ctx context.Context, id int64) (sqlc.Staff, error) {
	s, err := r.q.GetStaffByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Staff{}, domain.ErrUnauthorized
	}
	return s, err
}

func (r *Repos) GetStaffByBranchAndCode(ctx context.Context, branchID int64, staffCode string) (sqlc.Staff, error) {
	s, err := r.q.GetStaffByBranchAndCode(ctx, sqlc.GetStaffByBranchAndCodeParams{
		BranchID:  branchID,
		StaffCode: staffCode,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Staff{}, domain.ErrUnauthorized
	}
	return s, err
}

func (r *Repos) ListStaffForBranch(ctx context.Context, branchID int64) ([]sqlc.Staff, error) {
	return r.q.ListStaffForBranch(ctx, branchID)
}

func (r *Repos) ListActiveStaffForBranch(ctx context.Context, branchID int64) ([]sqlc.Staff, error) {
	return r.q.ListActiveStaffForBranch(ctx, branchID)
}

// ListStaffRosterForBranch returns active staff WITHOUT pin hashes (safe to
// surface to the manager/owner staff list).
func (r *Repos) ListStaffRosterForBranch(ctx context.Context, branchID int64) ([]sqlc.ListStaffRosterForBranchRow, error) {
	return r.q.ListStaffRosterForBranch(ctx, branchID)
}

func (r *Repos) CreateStaff(ctx context.Context, p sqlc.CreateStaffParams) (sqlc.Staff, error) {
	return r.q.CreateStaff(ctx, p)
}

func (r *Repos) UpdateStaffPIN(ctx context.Context, staffID int64, pinHash string) error {
	return r.q.UpdateStaffPIN(ctx, sqlc.UpdateStaffPINParams{ID: staffID, PinHash: pinHash})
}

func (r *Repos) UpdateStaffPINScoped(ctx context.Context, staffID, branchID int64, pinHash string) error {
	tag, err := r.db.Exec(ctx, `
UPDATE staff
SET pin_hash = $3,
    pin_version = pin_version + 1,
    token_version = token_version + 1
WHERE id = $1 AND branch_id = $2
`, staffID, branchID, pinHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUnauthorized
	}
	return nil
}

func (r *Repos) DeactivateStaff(ctx context.Context, staffID int64) error {
	return r.q.DeactivateStaff(ctx, staffID)
}

func (r *Repos) DeactivateStaffScoped(ctx context.Context, staffID, branchID int64) error {
	tag, err := r.db.Exec(ctx, `
UPDATE staff
SET is_active = FALSE,
    token_version = token_version + 1
WHERE id = $1 AND branch_id = $2
`, staffID, branchID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUnauthorized
	}
	return nil
}

func (r *Repos) CreateStaffSession(ctx context.Context, staffID, branchID int64, tokenHash, deviceName string, tokenVersion, pinVersion int32, expiresAt time.Time) (sqlc.StaffSession, error) {
	return r.q.CreateStaffSession(ctx, sqlc.CreateStaffSessionParams{
		StaffID:      staffID,
		BranchID:     branchID,
		TokenHash:    tokenHash,
		DeviceName:   deviceName,
		TokenVersion: tokenVersion,
		PinVersion:   pinVersion,
		ExpiresAt:    expiresAt,
	})
}

func (r *Repos) GetActiveStaffSessionByTokenHash(ctx context.Context, tokenHash string) (sqlc.StaffSession, error) {
	session, err := r.q.GetActiveStaffSessionByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.StaffSession{}, domain.ErrUnauthorized
	}
	return session, err
}

func (r *Repos) TouchStaffSession(ctx context.Context, sessionID uuid.UUID) error {
	return r.q.TouchStaffSession(ctx, sessionID)
}

func (r *Repos) RevokeStaffSessionsForStaff(ctx context.Context, staffID int64) error {
	return r.q.RevokeStaffSessionsForStaff(ctx, staffID)
}

func (r *Repos) UpdateBranchSessionTimeout(ctx context.Context, branchID int64, timeoutMinutes int16) error {
	return r.q.UpdateBranchSessionTimeout(ctx, sqlc.UpdateBranchSessionTimeoutParams{
		ID:                    branchID,
		SessionTimeoutMinutes: timeoutMinutes,
	})
}
