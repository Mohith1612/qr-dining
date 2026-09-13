package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) GetPlatformUserByEmail(ctx context.Context, email string) (sqlc.PlatformUser, error) {
	user, err := r.q.GetPlatformUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformUser{}, domain.ErrUnauthorized
	}
	return user, err
}

func (r *Repos) GetPlatformUserByID(ctx context.Context, id int64) (sqlc.PlatformUser, error) {
	user, err := r.q.GetPlatformUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformUser{}, domain.ErrUnauthorized
	}
	return user, err
}

func (r *Repos) ListPlatformUsers(ctx context.Context) ([]sqlc.PlatformUser, error) {
	return r.q.ListPlatformUsers(ctx)
}

func (r *Repos) ListPlatformRolesForUser(ctx context.Context, userID int64) ([]string, error) {
	return r.q.ListPlatformRolesForUser(ctx, userID)
}

func (r *Repos) UpsertPlatformUser(ctx context.Context, p sqlc.UpsertPlatformUserParams) (sqlc.PlatformUser, error) {
	return r.q.UpsertPlatformUser(ctx, p)
}

func (r *Repos) AddPlatformUserRole(ctx context.Context, userID int64, role string) error {
	return r.q.AddPlatformUserRole(ctx, sqlc.AddPlatformUserRoleParams{
		PlatformUserID: userID,
		Role:           role,
	})
}

func (r *Repos) CreatePlatformSession(ctx context.Context, p sqlc.CreatePlatformSessionParams) (sqlc.PlatformSession, error) {
	return r.q.CreatePlatformSession(ctx, p)
}

func (r *Repos) GetActivePlatformSessionByTokenHash(ctx context.Context, tokenHash string) (sqlc.PlatformSession, error) {
	session, err := r.q.GetActivePlatformSessionByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformSession{}, domain.ErrUnauthorized
	}
	return session, err
}

func (r *Repos) TouchPlatformSession(ctx context.Context, sessionID uuid.UUID) error {
	return r.q.TouchPlatformSession(ctx, sessionID)
}

func (r *Repos) RevokePlatformSession(ctx context.Context, sessionID uuid.UUID, userID int64) error {
	return r.q.RevokePlatformSession(ctx, sqlc.RevokePlatformSessionParams{
		ID:             sessionID,
		PlatformUserID: userID,
	})
}

func (r *Repos) ListPlatformOrganizations(ctx context.Context) ([]sqlc.Organization, error) {
	return r.q.ListPlatformOrganizations(ctx)
}

func (r *Repos) CreatePlatformOrganization(ctx context.Context, p sqlc.CreatePlatformOrganizationParams) (sqlc.Organization, error) {
	return r.q.CreatePlatformOrganization(ctx, p)
}

func (r *Repos) CreatePlatformRestaurant(ctx context.Context, p sqlc.CreatePlatformRestaurantParams) (sqlc.Restaurant, error) {
	return r.q.CreatePlatformRestaurant(ctx, p)
}

func (r *Repos) CreatePlatformBranch(ctx context.Context, p sqlc.CreatePlatformBranchParams) (sqlc.Branch, error) {
	return r.q.CreatePlatformBranch(ctx, p)
}

// UpdatePlatformBranch edits a branch's identity fields. A duplicate branch_code
// surfaces as domain.ErrDuplicateBranchCode for the handler to map to a 409.
func (r *Repos) UpdatePlatformBranch(ctx context.Context, id int64, name, timezone, branchCode, orderPrefix string) (sqlc.Branch, error) {
	b, err := r.q.UpdatePlatformBranch(ctx, sqlc.UpdatePlatformBranchParams{
		ID:          id,
		Name:        name,
		Timezone:    timezone,
		BranchCode:  branchCode,
		OrderPrefix: orderPrefix,
	})
	if err != nil {
		if isDuplicateError(err) {
			return sqlc.Branch{}, domain.ErrDuplicateBranchCode
		}
		return sqlc.Branch{}, err
	}
	return b, nil
}

func (r *Repos) CreateOrganizationBranchMembership(ctx context.Context, organizationID, branchID int64) error {
	return r.q.CreateOrganizationBranchMembership(ctx, sqlc.CreateOrganizationBranchMembershipParams{
		OrganizationID: organizationID,
		BranchID:       branchID,
	})
}

func (r *Repos) CreateOrganizationMember(ctx context.Context, organizationID, staffID int64, role string) (sqlc.OrganizationMember, error) {
	return r.q.CreateOrganizationMember(ctx, sqlc.CreateOrganizationMemberParams{
		OrganizationID: organizationID,
		StaffID:        staffID,
		Role:           role,
	})
}

func (r *Repos) CreatePlatformSupportSession(ctx context.Context, p sqlc.CreatePlatformSupportSessionParams) (sqlc.PlatformSupportSession, error) {
	return r.q.CreatePlatformSupportSession(ctx, p)
}

func (r *Repos) GetPlatformSupportSessionByID(ctx context.Context, id int64) (sqlc.PlatformSupportSession, error) {
	session, err := r.q.GetPlatformSupportSessionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PlatformSupportSession{}, domain.ErrTenantNotFound
	}
	return session, err
}

func (r *Repos) ListPlatformSupportSessions(ctx context.Context) ([]sqlc.PlatformSupportSession, error) {
	return r.q.ListPlatformSupportSessions(ctx)
}

func (r *Repos) ListPlatformAuditLog(ctx context.Context, p sqlc.ListPlatformAuditLogParams) ([]sqlc.PlatformAuditLog, error) {
	return r.q.ListPlatformAuditLog(ctx, p)
}

type PlatformAuditParams struct {
	PlatformUserID   int64
	Action           string
	TargetType       string
	TargetID         string
	OrganizationID   int64
	BranchID         int64
	SupportSessionID int64
	RequestID        string
	Payload          any
}

func (r *Repos) LogPlatformAudit(ctx context.Context, p PlatformAuditParams) {
	raw, err := json.Marshal(p.Payload)
	if err != nil {
		r.logger.Error().Err(err).Str("action", p.Action).Msg("platform_audit: marshal payload")
		return
	}
	err = r.q.InsertPlatformAuditLog(ctx, sqlc.InsertPlatformAuditLogParams{
		PlatformUserID:   pgtype.Int8{Int64: p.PlatformUserID, Valid: p.PlatformUserID != 0},
		Action:           p.Action,
		TargetType:       p.TargetType,
		TargetID:         p.TargetID,
		OrganizationID:   pgtype.Int8{Int64: p.OrganizationID, Valid: p.OrganizationID != 0},
		BranchID:         pgtype.Int8{Int64: p.BranchID, Valid: p.BranchID != 0},
		SupportSessionID: pgtype.Int8{Int64: p.SupportSessionID, Valid: p.SupportSessionID != 0},
		RequestID:        p.RequestID,
		Payload:          json.RawMessage(raw),
	})
	if err != nil {
		r.logger.Error().Err(err).Str("action", p.Action).Msg("platform_audit: insert")
	}
}
