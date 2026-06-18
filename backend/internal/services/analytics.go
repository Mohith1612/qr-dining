package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

const analyticsCacheTTL = 5 * time.Minute

type AnalyticsService struct {
	repos  *repository.Repos
	subSvc *SubscriptionService
	cache  *redisPkg.Cache
}

func NewAnalyticsService(repos *repository.Repos, subSvc *SubscriptionService, cache *redisPkg.Cache) *AnalyticsService {
	return &AnalyticsService{repos: repos, subSvc: subSvc, cache: cache}
}

func (s *AnalyticsService) checkAccess(ctx context.Context, restaurantID int64) error {
	// restaurantID 0 means tenant enforcement is disabled (local dev).
	if restaurantID == 0 {
		return nil
	}
	features, err := s.subSvc.GetPlanFeatures(ctx, restaurantID)
	if err != nil {
		return err
	}
	if !features.Analytics {
		return domain.ErrAnalyticsGated
	}
	return nil
}

func periodWindow(period string) (from, to time.Time) {
	now := time.Now().UTC()
	switch period {
	case "daily":
		return now.Add(-24 * time.Hour), now
	case "monthly":
		return now.Add(-30 * 24 * time.Hour), now
	default: // "weekly"
		return now.Add(-7 * 24 * time.Hour), now
	}
}

func cacheKey(branchID int64, period, metric string) string {
	return fmt.Sprintf("analytics:%d:%s:%s", branchID, period, metric)
}

func orgCacheKey(organizationID int64, period, metric string) string {
	return fmt.Sprintf("analytics:org:%d:%s:%s", organizationID, period, metric)
}

func (s *AnalyticsService) GetTopItems(ctx context.Context, restaurantID, branchID int64, period string) ([]sqlc.GetTopOrderedItemsRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := cacheKey(branchID, period, "top-items")
	var cached []sqlc.GetTopOrderedItemsRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetTopOrderedItems(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetTopOrderedItemsRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

func (s *AnalyticsService) GetBusyHours(ctx context.Context, restaurantID, branchID int64, period string) ([]sqlc.GetBusyHoursRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := cacheKey(branchID, period, "busy-hours")
	var cached []sqlc.GetBusyHoursRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetBusyHours(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetBusyHoursRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

func (s *AnalyticsService) GetOrderVolume(ctx context.Context, restaurantID, branchID int64, period string) ([]sqlc.GetOrderVolumeRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := cacheKey(branchID, period, "order-volume")
	var cached []sqlc.GetOrderVolumeRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetOrderVolume(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetOrderVolumeRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

func (s *AnalyticsService) GetOrganizationTopItems(ctx context.Context, restaurantID, organizationID int64, period string) ([]sqlc.GetOrganizationTopOrderedItemsRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := orgCacheKey(organizationID, period, "top-items")
	var cached []sqlc.GetOrganizationTopOrderedItemsRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetOrganizationTopOrderedItems(ctx, organizationID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetOrganizationTopOrderedItemsRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

func (s *AnalyticsService) GetOrganizationBusyHours(ctx context.Context, restaurantID, organizationID int64, period string) ([]sqlc.GetOrganizationBusyHoursRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := orgCacheKey(organizationID, period, "busy-hours")
	var cached []sqlc.GetOrganizationBusyHoursRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetOrganizationBusyHours(ctx, organizationID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetOrganizationBusyHoursRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

func (s *AnalyticsService) GetOrganizationOrderVolume(ctx context.Context, restaurantID, organizationID int64, period string) ([]sqlc.GetOrganizationOrderVolumeRow, error) {
	if err := s.checkAccess(ctx, restaurantID); err != nil {
		return nil, err
	}

	key := orgCacheKey(organizationID, period, "order-volume")
	var cached []sqlc.GetOrganizationOrderVolumeRow
	if hit, _ := s.cache.Get(ctx, key, &cached); hit {
		return cached, nil
	}

	from, to := periodWindow(period)
	rows, err := s.repos.GetOrganizationOrderVolume(ctx, organizationID, from, to)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []sqlc.GetOrganizationOrderVolumeRow{}
	}
	_ = s.cache.Set(ctx, key, rows, analyticsCacheTTL)
	return rows, nil
}

// IsAnalyticsGated returns true if the error is an analytics access gate.
func IsAnalyticsGated(err error) bool {
	return errors.Is(err, domain.ErrAnalyticsGated)
}
