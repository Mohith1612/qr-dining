package repository

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Staff performance analytics reads. Aggregation only — no writes.

func branchIDArg(branchID int64) pgtype.Int8 {
	return pgtype.Int8{Int64: branchID, Valid: true}
}

func (r *Repos) GetStaffOrderActivity(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetStaffOrderActivityRow, error) {
	return r.q.GetStaffOrderActivity(ctx, sqlc.GetStaffOrderActivityParams{
		BranchID: branchIDArg(branchID),
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetStaffAssistanceStats(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetStaffAssistanceStatsRow, error) {
	return r.q.GetStaffAssistanceStats(ctx, sqlc.GetStaffAssistanceStatsParams{
		BranchID: branchIDArg(branchID),
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetStaffSettlementStats(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetStaffSettlementStatsRow, error) {
	return r.q.GetStaffSettlementStats(ctx, sqlc.GetStaffSettlementStatsParams{
		BranchID: branchID,
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetStaffLoginStats(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetStaffLoginStatsRow, error) {
	return r.q.GetStaffLoginStats(ctx, sqlc.GetStaffLoginStatsParams{
		BranchID: branchID,
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetKitchenPerformance(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetKitchenPerformanceRow, error) {
	return r.q.GetKitchenPerformance(ctx, sqlc.GetKitchenPerformanceParams{
		BranchID: branchIDArg(branchID),
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetKitchenPeakThroughput(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetKitchenPeakThroughputRow, error) {
	return r.q.GetKitchenPeakThroughput(ctx, sqlc.GetKitchenPeakThroughputParams{
		BranchID: branchIDArg(branchID),
		FromTime: from,
		ToTime:   to,
	})
}

func (r *Repos) GetStaffDailyActivity(ctx context.Context, branchID int64, from, to time.Time) ([]sqlc.GetStaffDailyActivityRow, error) {
	return r.q.GetStaffDailyActivity(ctx, sqlc.GetStaffDailyActivityParams{
		BranchID: branchIDArg(branchID),
		FromTime: from,
		ToTime:   to,
	})
}
