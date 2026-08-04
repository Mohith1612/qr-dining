package services

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

type PromoService struct {
	repos *repository.Repos
}

func NewPromoService(repos *repository.Repos) *PromoService {
	return &PromoService{repos: repos}
}

type ValidatePromoRequest struct {
	BranchID   int64
	Code       string
	OrderTotal float64
	PhoneE164  *string
}

type ValidatePromoResult struct {
	PromoID        int64
	DiscountAmount float64
	Description    string
	// NormalizedPhone is the canonical form of the caller's phone, as used for
	// the per-phone cap check. Callers that record a redemption MUST persist
	// this value rather than the raw request string, or the cap check will
	// never match what was stored.
	NormalizedPhone *string
}

// ValidatePromo checks a promo code for validity and returns the discount. No DB writes.
// Accepts repos so it can be called with a TX-backed repos inside a transaction.
func (s *PromoService) ValidatePromo(ctx context.Context, repos *repository.Repos, req ValidatePromoRequest) (ValidatePromoResult, error) {
	promo, err := repos.GetPromoByCodeForUpdate(ctx, req.BranchID, req.Code)
	if err != nil {
		return ValidatePromoResult{}, err // ErrPromoNotFound propagated as-is
	}
	if req.PhoneE164 != nil {
		normalized := promoNormalizePhone(*req.PhoneE164)
		req.PhoneE164 = &normalized
	}

	// A per-phone usage limit is only enforceable with a phone. Refuse to apply
	// such a promo without one so the limit can't be silently bypassed.
	if promo.UsesPerPhone > 0 && req.PhoneE164 == nil {
		return ValidatePromoResult{}, domain.ErrPromoPhoneRequired
	}

	// Check minimum order amount.
	minF, _ := promo.MinOrderAmount.Float64Value()
	if minF.Valid && req.OrderTotal < minF.Float64 {
		return ValidatePromoResult{}, domain.ErrMinOrderNotMet
	}

	// Check global redemption cap.
	if promo.MaxUses.Valid {
		if promo.RedeemedCount >= promo.MaxUses.Int32 {
			return ValidatePromoResult{}, domain.ErrPromoExhausted
		}
	}

	// Check per-phone cap.
	if req.PhoneE164 != nil && promo.UsesPerPhone > 0 {
		count, err := repos.CountPromoRedemptionsByPhone(ctx, promo.ID, *req.PhoneE164)
		if err != nil {
			return ValidatePromoResult{}, err
		}
		if count >= int64(promo.UsesPerPhone) {
			return ValidatePromoResult{}, domain.ErrPromoAlreadyUsed
		}
	}

	discount := computeDiscount(promo, req.OrderTotal)
	desc := ""
	if promo.Description.Valid {
		desc = promo.Description.String
	}

	return ValidatePromoResult{
		PromoID:         promo.ID,
		DiscountAmount:  discount,
		Description:     desc,
		NormalizedPhone: req.PhoneE164,
	}, nil
}

func NormalizePromoPhone(phone string) string {
	return promoNormalizePhone(phone)
}

// promoNormalizePhone reduces a phone to a single canonical form so the
// per-phone redemption cap cannot be evaded by reformatting the same number.
// It keeps digits only and always re-applies the leading "+", so every variant
// of one number ("+919876543210", "+91 98765 43210", "(+91)9876543210",
// "919876543210") collapses to the same string. Anchoring the "+" to index 0 —
// as an earlier version did — is not sufficient: a leading "(" pushed the "+"
// off index 0 and silently produced a second, cap-evading canonical form.
func promoNormalizePhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	return "+" + digits
}

func computeDiscount(promo sqlc.Promo, orderTotal float64) float64 {
	v, _ := promo.Value.Float64Value()
	value := v.Float64
	switch promo.Type {
	case sqlc.PromoTypeFlatAmount:
		if value > orderTotal {
			return round2(orderTotal)
		}
		return round2(value)
	case sqlc.PromoTypePercentage:
		return round2(orderTotal * value / 100)
	}
	return 0
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type CreatePromoRequest struct {
	BranchID        int64
	Code            string
	Type            sqlc.PromoType
	Value           float64
	MinOrderAmount  float64
	MaxUses         *int32
	UsesPerPhone    int32
	ValidFrom       time.Time
	ValidUntil      time.Time
	TimeWindowStart *time.Duration
	TimeWindowEnd   *time.Duration
	Description     *string
	CreatedBy       *int64
}

func (s *PromoService) CreatePromo(ctx context.Context, req CreatePromoRequest) (sqlc.Promo, error) {
	return s.repos.CreatePromo(ctx, repository.CreatePromoParams{
		BranchID:        req.BranchID,
		Code:            req.Code,
		Type:            req.Type,
		Value:           req.Value,
		MinOrderAmount:  req.MinOrderAmount,
		MaxUses:         req.MaxUses,
		UsesPerPhone:    req.UsesPerPhone,
		ValidFrom:       req.ValidFrom,
		ValidUntil:      req.ValidUntil,
		TimeWindowStart: req.TimeWindowStart,
		TimeWindowEnd:   req.TimeWindowEnd,
		Description:     req.Description,
		CreatedBy:       req.CreatedBy,
	})
}

func (s *PromoService) ListPromosForBranch(ctx context.Context, branchID int64) ([]sqlc.Promo, error) {
	return s.repos.ListPromosForBranch(ctx, branchID)
}

func (s *PromoService) DeactivatePromo(ctx context.Context, promoID, branchID int64) error {
	return s.repos.DeactivatePromo(ctx, promoID, branchID)
}

func (s *PromoService) ActivatePromo(ctx context.Context, promoID, branchID int64) error {
	return s.repos.ActivatePromo(ctx, promoID, branchID)
}

// UpdatePromoRequest mirrors CreatePromoRequest minus the immutable code/type.
type UpdatePromoRequest struct {
	PromoID         int64
	BranchID        int64
	Value           float64
	MinOrderAmount  float64
	MaxUses         *int32
	UsesPerPhone    int32
	ValidFrom       time.Time
	ValidUntil      time.Time
	TimeWindowStart *time.Duration
	TimeWindowEnd   *time.Duration
	Description     *string
}

func (s *PromoService) UpdatePromo(ctx context.Context, req UpdatePromoRequest) (sqlc.Promo, error) {
	return s.repos.UpdatePromo(ctx, repository.UpdatePromoParams{
		PromoID:         req.PromoID,
		BranchID:        req.BranchID,
		Value:           req.Value,
		MinOrderAmount:  req.MinOrderAmount,
		MaxUses:         req.MaxUses,
		UsesPerPhone:    req.UsesPerPhone,
		ValidFrom:       req.ValidFrom,
		ValidUntil:      req.ValidUntil,
		TimeWindowStart: req.TimeWindowStart,
		TimeWindowEnd:   req.TimeWindowEnd,
		Description:     req.Description,
	})
}
