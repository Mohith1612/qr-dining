package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreatePromoParams struct {
	BranchID        int64
	Code            string
	Type            sqlc.PromoType
	Value           float64
	MinOrderAmount  float64
	MaxUses         *int32
	UsesPerPhone    int32
	ValidFrom       time.Time
	ValidUntil      time.Time
	TimeWindowStart *time.Duration // seconds since midnight, nil = all day
	TimeWindowEnd   *time.Duration
	Description     *string
	CreatedBy       *int64
}

func (r *Repos) GetPromoByCode(ctx context.Context, branchID int64, code string) (sqlc.Promo, error) {
	p, err := r.q.GetPromoByCode(ctx, sqlc.GetPromoByCodeParams{
		BranchID: branchID,
		Lower:    code,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Promo{}, domain.ErrPromoNotFound
	}
	return p, err
}

func (r *Repos) GetPromoByCodeForUpdate(ctx context.Context, branchID int64, code string) (sqlc.Promo, error) {
	p, err := r.q.GetPromoByCodeForUpdate(ctx, sqlc.GetPromoByCodeForUpdateParams{
		BranchID: branchID,
		Lower:    code,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Promo{}, domain.ErrPromoNotFound
	}
	return p, err
}

func (r *Repos) GetPromoByID(ctx context.Context, promoID int64) (sqlc.Promo, error) {
	p, err := r.q.GetPromoByID(ctx, promoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Promo{}, domain.ErrPromoNotFound
	}
	return p, err
}

func (r *Repos) CountPromoRedemptions(ctx context.Context, promoID int64) (int64, error) {
	return r.q.CountPromoRedemptions(ctx, promoID)
}

func (r *Repos) CountPromoRedemptionsByPhone(ctx context.Context, promoID int64, phone string) (int64, error) {
	return r.q.CountPromoRedemptionsByPhone(ctx, sqlc.CountPromoRedemptionsByPhoneParams{
		PromoID:   promoID,
		PhoneE164: pgtype.Text{String: phone, Valid: true},
	})
}

// CreatePromoRedemptionForPayment records a redemption tied to a payment
// (promo applied at payment initiation rather than order placement).
func (r *Repos) CreatePromoRedemptionForPayment(ctx context.Context, promoID, paymentID int64, phone *string) (sqlc.PromoRedemption, error) {
	p := sqlc.CreatePromoRedemptionForPaymentParams{
		PromoID:   promoID,
		PaymentID: pgtype.Int8{Int64: paymentID, Valid: true},
	}
	if phone != nil {
		p.PhoneE164 = pgtype.Text{String: *phone, Valid: true}
	}
	return r.q.CreatePromoRedemptionForPayment(ctx, p)
}

func (r *Repos) IncrementPromoRedemptionCount(ctx context.Context, promoID int64) error {
	return r.q.IncrementPromoRedemptionCount(ctx, promoID)
}

func (r *Repos) CreatePromo(ctx context.Context, p CreatePromoParams) (sqlc.Promo, error) {
	var value, minOrder pgtype.Numeric
	_ = value.Scan(numericStr(p.Value))
	_ = minOrder.Scan(numericStr(p.MinOrderAmount))

	params := sqlc.CreatePromoParams{
		BranchID:       p.BranchID,
		Code:           p.Code,
		Type:           p.Type,
		Value:          value,
		MinOrderAmount: minOrder,
		UsesPerPhone:   p.UsesPerPhone,
		ValidFrom:      p.ValidFrom,
		ValidUntil:     p.ValidUntil,
	}
	if p.MaxUses != nil {
		params.MaxUses = pgtype.Int4{Int32: *p.MaxUses, Valid: true}
	}
	if p.TimeWindowStart != nil {
		// pgtype.Time uses microseconds since midnight
		params.TimeWindowStart = pgtype.Time{Microseconds: p.TimeWindowStart.Microseconds(), Valid: true}
		params.TimeWindowEnd = pgtype.Time{Microseconds: p.TimeWindowEnd.Microseconds(), Valid: true}
	}
	if p.Description != nil {
		params.Description = pgtype.Text{String: *p.Description, Valid: true}
	}
	if p.CreatedBy != nil {
		params.CreatedBy = pgtype.Int8{Int64: *p.CreatedBy, Valid: true}
	}
	return r.q.CreatePromo(ctx, params)
}

func (r *Repos) ListPromosForBranch(ctx context.Context, branchID int64) ([]sqlc.Promo, error) {
	return r.q.ListPromosForBranch(ctx, branchID)
}

func (r *Repos) DeactivatePromo(ctx context.Context, promoID, branchID int64) error {
	return r.q.DeactivatePromo(ctx, sqlc.DeactivatePromoParams{ID: promoID, BranchID: branchID})
}

func numericStr(v float64) string {
	return fmt.Sprintf("%.2f", math.Round(v*100)/100)
}
