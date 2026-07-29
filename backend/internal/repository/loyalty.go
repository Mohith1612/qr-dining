package repository

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Loyalty program / accounts / ledger access (migration 000035).

func (r *Repos) UpsertLoyaltyProgram(ctx context.Context, orgID int64, isActive bool, earnRatePoints int64, earnRateAmount pgtype.Numeric, updatedByStaffID int64) (sqlc.OrganizationLoyaltyProgram, error) {
	return r.q.UpsertLoyaltyProgram(ctx, sqlc.UpsertLoyaltyProgramParams{
		OrganizationID:   orgID,
		IsActive:         isActive,
		EarnRatePoints:   earnRatePoints,
		EarnRateAmount:   earnRateAmount,
		UpdatedByStaffID: pgtype.Int8{Int64: updatedByStaffID, Valid: updatedByStaffID != 0},
	})
}

func (r *Repos) GetLoyaltyProgram(ctx context.Context, orgID int64) (sqlc.OrganizationLoyaltyProgram, error) {
	return r.q.GetLoyaltyProgram(ctx, orgID)
}

func (r *Repos) UpsertLoyaltyAccountForEarn(ctx context.Context, customerID, orgID, points, visitIncrement int64, amount pgtype.Numeric) (sqlc.CustomerLoyaltyAccount, error) {
	return r.q.UpsertLoyaltyAccountForEarn(ctx, sqlc.UpsertLoyaltyAccountForEarnParams{
		CustomerID:     customerID,
		OrganizationID: orgID,
		Points:         points,
		VisitIncrement: visitIncrement,
		Amount:         amount,
	})
}

// LoyaltyTransactionParams carries optional links for a ledger row.
type LoyaltyTransactionParams struct {
	AccountID int64
	Type      string // earn | redeem | adjustment
	Points    int64  // signed
	Amount    pgtype.Numeric
	PaymentID int64     // 0 = none
	SessionID uuid.UUID // uuid.Nil = none
	ActorType string    // system | staff
	StaffID   int64     // 0 = none
	Reason    string
}

func (r *Repos) InsertLoyaltyTransaction(ctx context.Context, p LoyaltyTransactionParams) (sqlc.CustomerLoyaltyTransaction, error) {
	return r.q.InsertLoyaltyTransaction(ctx, sqlc.InsertLoyaltyTransactionParams{
		AccountID:            p.AccountID,
		Type:                 p.Type,
		Points:               p.Points,
		Amount:               p.Amount,
		PaymentID:            pgtype.Int8{Int64: p.PaymentID, Valid: p.PaymentID != 0},
		SessionID:            pgtype.UUID{Bytes: p.SessionID, Valid: p.SessionID != uuid.Nil},
		PerformedByActorType: p.ActorType,
		PerformedByStaffID:   pgtype.Int8{Int64: p.StaffID, Valid: p.StaffID != 0},
		Reason:               p.Reason,
	})
}

func (r *Repos) CountEarnTransactionsForAccountSession(ctx context.Context, accountID int64, sessionID uuid.UUID) (int64, error) {
	return r.q.CountEarnTransactionsForAccountSession(ctx, sqlc.CountEarnTransactionsForAccountSessionParams{
		AccountID: accountID,
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: sessionID != uuid.Nil},
	})
}

func (r *Repos) DeductLoyaltyPoints(ctx context.Context, accountID, orgID, points int64) (sqlc.CustomerLoyaltyAccount, error) {
	return r.q.DeductLoyaltyPoints(ctx, sqlc.DeductLoyaltyPointsParams{
		Points:         points,
		AccountID:      accountID,
		OrganizationID: orgID,
	})
}

func (r *Repos) AdjustLoyaltyPoints(ctx context.Context, accountID, orgID, pointsDelta int64) (sqlc.CustomerLoyaltyAccount, error) {
	return r.q.AdjustLoyaltyPoints(ctx, sqlc.AdjustLoyaltyPointsParams{
		PointsDelta:    pointsDelta,
		AccountID:      accountID,
		OrganizationID: orgID,
	})
}

func (r *Repos) GetLoyaltyAccountByID(ctx context.Context, id, orgID int64) (sqlc.CustomerLoyaltyAccount, error) {
	return r.q.GetLoyaltyAccountByID(ctx, sqlc.GetLoyaltyAccountByIDParams{ID: id, OrganizationID: orgID})
}

func (r *Repos) GetLoyaltyAccountByCustomer(ctx context.Context, customerID, orgID int64) (sqlc.CustomerLoyaltyAccount, error) {
	return r.q.GetLoyaltyAccountByCustomer(ctx, sqlc.GetLoyaltyAccountByCustomerParams{CustomerID: customerID, OrganizationID: orgID})
}

func (r *Repos) ListLoyaltyTransactions(ctx context.Context, accountID int64, limit, offset int32) ([]sqlc.CustomerLoyaltyTransaction, error) {
	return r.q.ListLoyaltyTransactions(ctx, sqlc.ListLoyaltyTransactionsParams{AccountID: accountID, Limit: limit, Offset: offset})
}

func (r *Repos) GetLoyaltyPointsSummary(ctx context.Context, orgID int64, from, to time.Time) (sqlc.GetLoyaltyPointsSummaryRow, error) {
	return r.q.GetLoyaltyPointsSummary(ctx, sqlc.GetLoyaltyPointsSummaryParams{OrganizationID: orgID, FromTime: from, ToTime: to})
}

func (r *Repos) GetLoyaltyParticipation(ctx context.Context, orgID int64, from, to time.Time) (sqlc.GetLoyaltyParticipationRow, error) {
	return r.q.GetLoyaltyParticipation(ctx, sqlc.GetLoyaltyParticipationParams{OrganizationID: orgID, FromTime: from, ToTime: to})
}

func (r *Repos) CountLoyaltyAccounts(ctx context.Context, orgID int64) (int64, error) {
	return r.q.CountLoyaltyAccounts(ctx, orgID)
}

func (r *Repos) GetTopLoyaltyCustomers(ctx context.Context, orgID int64) ([]sqlc.GetTopLoyaltyCustomersRow, error) {
	return r.q.GetTopLoyaltyCustomers(ctx, orgID)
}
