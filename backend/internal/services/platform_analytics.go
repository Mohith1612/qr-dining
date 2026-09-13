package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

const platformAnalyticsCacheTTL = 5 * time.Minute

// PlatformAnalyticsService computes cross-tenant aggregations on demand (Redis-cached).
// orgID == nil means platform-wide; a value scopes to one organization.
type PlatformAnalyticsService struct {
	repos *repository.Repos
	cache *redis.Cache
}

func NewPlatformAnalyticsService(repos *repository.Repos, cache *redis.Cache) *PlatformAnalyticsService {
	return &PlatformAnalyticsService{repos: repos, cache: cache}
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type DayRevenue struct {
	Day     string `json:"day"`
	Count   int64  `json:"count"`
	Revenue string `json:"revenue"`
}

type BranchRevenue struct {
	BranchID   int64  `json:"branch_id"`
	BranchName string `json:"branch_name"`
	BranchCode string `json:"branch_code"`
	Count      int64  `json:"count"`
	Revenue    string `json:"revenue"`
}

type OrgRevenue struct {
	OrganizationID   int64  `json:"organization_id"`
	OrganizationCode string `json:"organization_code"`
	Count            int64  `json:"count"`
	Revenue          string `json:"revenue"`
}

type UsageReport struct {
	Period                 string     `json:"period"`
	SessionsPerDay         []DayCount `json:"sessions_per_day"`
	OrdersPerDay           []DayCount `json:"orders_per_day"`
	PaymentsPerDay         []DayCount `json:"payments_per_day"`
	ParticipantJoinsPerDay []DayCount `json:"participant_joins_per_day"`
	ActiveBranches         int64      `json:"active_branches"`
	ActiveDiners           int64      `json:"active_diners"`
}

type RevenueReport struct {
	Period          string          `json:"period"`
	GMV             string          `json:"gmv"`
	RevenuePerDay   []DayRevenue    `json:"revenue_per_day"`
	RevenueByBranch []BranchRevenue `json:"revenue_by_branch"`
	RevenueByOrg    []OrgRevenue    `json:"revenue_by_org"`
}

type HealthReport struct {
	Period                string     `json:"period"`
	AuthzDenialsPerDay    []DayCount `json:"authz_denials_per_day"`
	WebhookFailuresPerDay []DayCount `json:"webhook_failures_per_day"`
}

// NormalizePeriod clamps to the supported set; default daily.
func NormalizePeriod(period string) string {
	switch period {
	case "weekly", "monthly":
		return period
	default:
		return "daily"
	}
}

func windowForPeriod(period string) (time.Time, time.Time) {
	to := time.Now().UTC()
	switch period {
	case "weekly":
		return to.AddDate(0, 0, -7), to
	case "monthly":
		return to.AddDate(0, 0, -30), to
	default:
		return to.AddDate(0, 0, -1), to
	}
}

func (s *PlatformAnalyticsService) GetUsage(ctx context.Context, orgID *int64, period string) (UsageReport, error) {
	period = NormalizePeriod(period)
	key := fmt.Sprintf("platform_analytics:usage:%s:%s", orgScope(orgID), period)
	var cached UsageReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	from, to := windowForPeriod(period)

	sessions, err := s.repos.PlatformSessionsPerDay(ctx, orgID, from, to)
	if err != nil {
		return UsageReport{}, err
	}
	orders, err := s.repos.PlatformOrdersPerDay(ctx, orgID, from, to)
	if err != nil {
		return UsageReport{}, err
	}
	payments, err := s.repos.PlatformPaymentsPerDay(ctx, orgID, from, to)
	if err != nil {
		return UsageReport{}, err
	}
	joins, err := s.repos.PlatformParticipantJoinsPerDay(ctx, orgID, from, to)
	if err != nil {
		return UsageReport{}, err
	}
	activeBranches, err := s.repos.PlatformActiveBranches(ctx, orgID, from, to)
	if err != nil {
		return UsageReport{}, err
	}
	activeDiners, err := s.repos.PlatformActiveDiners(ctx, orgID)
	if err != nil {
		return UsageReport{}, err
	}

	report := UsageReport{
		Period:                 period,
		SessionsPerDay:         dayCounts(sessions, func(r sqlc.PlatformSessionsPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
		OrdersPerDay:           dayCounts(orders, func(r sqlc.PlatformOrdersPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
		PaymentsPerDay:         dayCounts(payments, func(r sqlc.PlatformPaymentsPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
		ParticipantJoinsPerDay: dayCounts(joins, func(r sqlc.PlatformParticipantJoinsPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
		ActiveBranches:         activeBranches,
		ActiveDiners:           activeDiners,
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

func (s *PlatformAnalyticsService) GetRevenue(ctx context.Context, orgID *int64, period string) (RevenueReport, error) {
	period = NormalizePeriod(period)
	key := fmt.Sprintf("platform_analytics:revenue:%s:%s", orgScope(orgID), period)
	var cached RevenueReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	from, to := windowForPeriod(period)

	gmv, err := s.repos.PlatformGMV(ctx, orgID, from, to)
	if err != nil {
		return RevenueReport{}, err
	}
	perDay, err := s.repos.PlatformRevenuePerDay(ctx, orgID, from, to)
	if err != nil {
		return RevenueReport{}, err
	}
	byBranch, err := s.repos.PlatformRevenueByBranch(ctx, orgID, from, to)
	if err != nil {
		return RevenueReport{}, err
	}
	byOrg, err := s.repos.PlatformRevenueByOrg(ctx, orgID, from, to)
	if err != nil {
		return RevenueReport{}, err
	}

	report := RevenueReport{
		Period:          period,
		GMV:             gmv,
		RevenuePerDay:   make([]DayRevenue, 0, len(perDay)),
		RevenueByBranch: make([]BranchRevenue, 0, len(byBranch)),
		RevenueByOrg:    make([]OrgRevenue, 0, len(byOrg)),
	}
	for _, r := range perDay {
		report.RevenuePerDay = append(report.RevenuePerDay, DayRevenue{Day: dayString(r.Day), Count: r.Count, Revenue: r.Revenue})
	}
	for _, r := range byBranch {
		report.RevenueByBranch = append(report.RevenueByBranch, BranchRevenue{
			BranchID: r.BranchID, BranchName: r.BranchName, BranchCode: r.BranchCode, Count: r.Count, Revenue: r.Revenue,
		})
	}
	for _, r := range byOrg {
		report.RevenueByOrg = append(report.RevenueByOrg, OrgRevenue{
			OrganizationID: r.OrganizationID, OrganizationCode: r.OrganizationCode, Count: r.Count, Revenue: r.Revenue,
		})
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

func (s *PlatformAnalyticsService) GetHealth(ctx context.Context, orgID *int64, period string) (HealthReport, error) {
	period = NormalizePeriod(period)
	key := fmt.Sprintf("platform_analytics:health:%s:%s", orgScope(orgID), period)
	var cached HealthReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	from, to := windowForPeriod(period)

	denials, err := s.repos.PlatformAuthzDenialsPerDay(ctx, orgID, from, to)
	if err != nil {
		return HealthReport{}, err
	}
	webhookFailures, err := s.repos.PlatformWebhookFailuresPerDay(ctx, orgID, from, to)
	if err != nil {
		return HealthReport{}, err
	}
	report := HealthReport{
		Period:                period,
		AuthzDenialsPerDay:    dayCounts(denials, func(r sqlc.PlatformAuthzDenialsPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
		WebhookFailuresPerDay: dayCounts(webhookFailures, func(r sqlc.PlatformWebhookFailuresPerDayRow) (pgtype.Date, int64) { return r.Day, r.Count }),
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

func dayCounts[T any](rows []T, get func(T) (pgtype.Date, int64)) []DayCount {
	out := make([]DayCount, 0, len(rows))
	for _, r := range rows {
		d, c := get(r)
		out = append(out, DayCount{Day: dayString(d), Count: c})
	}
	return out
}

func dayString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// cacheGet/cacheSet tolerate a nil cache (used in tests) and never fail the request.
func (s *PlatformAnalyticsService) cacheGet(ctx context.Context, key string, dst any) bool {
	if s.cache == nil {
		return false
	}
	hit, _ := s.cache.Get(ctx, key, dst)
	return hit
}

func (s *PlatformAnalyticsService) cacheSet(ctx context.Context, key string, v any) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Set(ctx, key, v, platformAnalyticsCacheTTL)
}

func orgScope(orgID *int64) string {
	if orgID == nil {
		return "all"
	}
	return fmt.Sprintf("org%d", *orgID)
}
