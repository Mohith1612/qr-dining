package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) ListPlans(ctx context.Context) ([]sqlc.SubscriptionPlan, error) {
	return r.q.ListPlans(ctx)
}

func (r *Repos) GetPlanByTier(ctx context.Context, tier sqlc.PlanTier) (sqlc.SubscriptionPlan, error) {
	p, err := r.q.GetPlanByTier(ctx, tier)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SubscriptionPlan{}, domain.ErrPlanNotFound
	}
	return p, err
}

func (r *Repos) GetSubscriptionByRestaurant(ctx context.Context, restaurantID int64) (sqlc.GetSubscriptionByRestaurantRow, error) {
	s, err := r.q.GetSubscriptionByRestaurant(ctx, restaurantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.GetSubscriptionByRestaurantRow{}, domain.ErrPlanNotFound
	}
	return s, err
}

func (r *Repos) UpsertSubscription(ctx context.Context, restaurantID, planID int64, status string, trialEndsAt *time.Time) (sqlc.RestaurantSubscription, error) {
	var trialPG pgtype.Timestamptz
	if trialEndsAt != nil {
		trialPG = pgtype.Timestamptz{Time: *trialEndsAt, Valid: true}
	}
	return r.q.UpsertSubscription(ctx, sqlc.UpsertSubscriptionParams{
		RestaurantID:       restaurantID,
		PlanID:             planID,
		Status:             status,
		TrialEndsAt:        trialPG,
		CurrentPeriodStart: time.Now(),
	})
}

// PlanFeatures is decoded from subscription_plans.features_json.
type PlanFeatures struct {
	MaxBranches int  `json:"max_branches"` // -1 = unlimited
	MaxTables   int  `json:"max_tables"`   // -1 = unlimited
	Analytics   bool `json:"analytics"`
	MultiBranch bool `json:"multi_branch"`
}

func DecodePlanFeatures(raw json.RawMessage) PlanFeatures {
	var f PlanFeatures
	// Default safe values for free plan when decode fails.
	f.MaxBranches = 1
	f.MaxTables = 10
	_ = json.Unmarshal(raw, &f)
	return f
}
