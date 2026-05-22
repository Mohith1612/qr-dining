package services

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

type SubscriptionService struct {
	repos *repository.Repos
}

func NewSubscriptionService(repos *repository.Repos) *SubscriptionService {
	return &SubscriptionService{repos: repos}
}

// GetPlanFeatures returns the decoded feature flags for the restaurant's active plan.
// Returns a safe free-tier default if no subscription is found.
func (s *SubscriptionService) GetPlanFeatures(ctx context.Context, restaurantID int64) (repository.PlanFeatures, error) {
	sub, err := s.repos.GetSubscriptionByRestaurant(ctx, restaurantID)
	if err != nil {
		if errors.Is(err, domain.ErrPlanNotFound) {
			// No subscription — treat as free tier with no analytics.
			return repository.PlanFeatures{MaxBranches: 1, MaxTables: 10}, nil
		}
		return repository.PlanFeatures{}, err
	}
	return repository.DecodePlanFeatures(sub.PlanFeaturesJson), nil
}
