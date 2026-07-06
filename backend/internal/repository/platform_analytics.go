package repository

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

// Platform (cross-tenant) analytics repo wrappers. orgID nil = platform-wide.

func (r *Repos) PlatformSessionsPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformSessionsPerDayRow, error) {
	return r.q.PlatformSessionsPerDay(ctx, sqlc.PlatformSessionsPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformOrdersPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformOrdersPerDayRow, error) {
	return r.q.PlatformOrdersPerDay(ctx, sqlc.PlatformOrdersPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformPaymentsPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformPaymentsPerDayRow, error) {
	return r.q.PlatformPaymentsPerDay(ctx, sqlc.PlatformPaymentsPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformParticipantJoinsPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformParticipantJoinsPerDayRow, error) {
	return r.q.PlatformParticipantJoinsPerDay(ctx, sqlc.PlatformParticipantJoinsPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformActiveBranches(ctx context.Context, orgID *int64, from, to time.Time) (int64, error) {
	return r.q.PlatformActiveBranches(ctx, sqlc.PlatformActiveBranchesParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformActiveDiners(ctx context.Context, orgID *int64) (int64, error) {
	return r.q.PlatformActiveDiners(ctx, int8FromPtr(orgID))
}

func (r *Repos) PlatformGMV(ctx context.Context, orgID *int64, from, to time.Time) (string, error) {
	return r.q.PlatformGMV(ctx, sqlc.PlatformGMVParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformRevenuePerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformRevenuePerDayRow, error) {
	return r.q.PlatformRevenuePerDay(ctx, sqlc.PlatformRevenuePerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformRevenueByBranch(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformRevenueByBranchRow, error) {
	return r.q.PlatformRevenueByBranch(ctx, sqlc.PlatformRevenueByBranchParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformRevenueByOrg(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformRevenueByOrgRow, error) {
	return r.q.PlatformRevenueByOrg(ctx, sqlc.PlatformRevenueByOrgParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformAuthzDenialsPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformAuthzDenialsPerDayRow, error) {
	return r.q.PlatformAuthzDenialsPerDay(ctx, sqlc.PlatformAuthzDenialsPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}

func (r *Repos) PlatformWebhookFailuresPerDay(ctx context.Context, orgID *int64, from, to time.Time) ([]sqlc.PlatformWebhookFailuresPerDayRow, error) {
	return r.q.PlatformWebhookFailuresPerDay(ctx, sqlc.PlatformWebhookFailuresPerDayParams{OrganizationID: int8FromPtr(orgID), FromTime: from, ToTime: to})
}
