package services

import (
	"context"
	"errors"
	"sort"

	"github.com/Mohith1612/qr-dining/internal/domain"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

// StaffAnalyticsService derives staff performance metrics from EXISTING
// operational data — event_log rows (order status transitions and assistance
// ack/resolve already carry staff actor ids), payments (settled_by_staff_id),
// and staff_sessions (logins/activity). It captures nothing new.
//
// Definition note: the system has no waiter↔session assignment concept (and
// this service does not invent one). "Sessions handled" therefore means
// "distinct sessions the staff member touched" (served an order in, or settled
// a payment for) — operational activity, not an assignment count.
//
// Access is gated per branch by the analytics.staff_performance entitlement
// AND the staff_performance_analytics platform flag (both default off).
type StaffAnalyticsService struct {
	repos *repository.Repos
	gate  *FeatureGate
	cache *redisPkg.Cache
}

func NewStaffAnalyticsService(repos *repository.Repos, gate *FeatureGate, cache *redisPkg.Cache) *StaffAnalyticsService {
	return &StaffAnalyticsService{repos: repos, gate: gate, cache: cache}
}

func (s *StaffAnalyticsService) checkAccess(ctx context.Context, branchID int64) error {
	enabled, err := s.gate.BranchEnabled(ctx, branchID, EntitlementStaffPerformanceAnalytics, FlagStaffPerformanceAnalytics)
	if err != nil {
		return err
	}
	if !enabled {
		return domain.ErrStaffAnalyticsDisabled
	}
	return nil
}

// WaiterPerformanceRow is per-staff front-of-house activity. Rows include any
// staff member with activity in the window (role included so callers can filter).
type WaiterPerformanceRow struct {
	StaffID              int64   `json:"staff_id"`
	StaffName            string  `json:"staff_name"`
	StaffRole            string  `json:"staff_role"`
	SessionsHandled      int64   `json:"sessions_handled"`
	TablesServed         int64   `json:"tables_served"`
	OrdersAssisted       int64   `json:"orders_assisted"`
	OrdersServed         int64   `json:"orders_served"`
	AssistanceAccepted   int64   `json:"assistance_accepted"`
	AssistanceResolved   int64   `json:"assistance_resolved"`
	AvgResponseSeconds   float64 `json:"avg_response_seconds"`
	PaymentsSettled      int64   `json:"payments_settled"`
	AvgSettlementSeconds float64 `json:"avg_settlement_seconds"`
	LoginCount           int64   `json:"login_count"`
	ActiveSeconds        float64 `json:"active_seconds"`
}

// KitchenPerformanceRow is per-staff kitchen throughput, attributed to whoever
// marked the order ready. Shared kitchen logins blur this attribution.
type KitchenPerformanceRow struct {
	StaffID           int64   `json:"staff_id"`
	StaffName         string  `json:"staff_name"`
	OrdersCompleted   int64   `json:"orders_completed"`
	AvgPrepSeconds    float64 `json:"avg_prep_seconds"`
	PeakOrdersPerHour int64   `json:"peak_orders_per_hour"`
}

// StaffDailyActivityRow is the manager utilization summary: one row per staff
// member per UTC day with activity.
type StaffDailyActivityRow struct {
	StaffID       int64   `json:"staff_id"`
	StaffName     string  `json:"staff_name"`
	StaffRole     string  `json:"staff_role"`
	Day           string  `json:"day"` // YYYY-MM-DD (UTC)
	EventCount    int64   `json:"event_count"`
	LoginCount    int64   `json:"login_count"`
	ActiveSeconds float64 `json:"active_seconds"`
}

// BranchStaffPerformance bundles all three views (platform operator read).
type BranchStaffPerformance struct {
	BranchID int64                   `json:"branch_id"`
	Period   string                  `json:"period"`
	Waiters  []WaiterPerformanceRow  `json:"waiters"`
	Kitchen  []KitchenPerformanceRow `json:"kitchen"`
	Summary  []StaffDailyActivityRow `json:"summary"`
}

// GetWaiterPerformance returns per-staff front-of-house metrics for the branch.
func (s *StaffAnalyticsService) GetWaiterPerformance(ctx context.Context, branchID int64, period string) ([]WaiterPerformanceRow, error) {
	if err := s.checkAccess(ctx, branchID); err != nil {
		return nil, err
	}
	key := cacheKey(branchID, period, "staff-waiters")
	var cached []WaiterPerformanceRow
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	rows, err := s.waiterPerformance(ctx, branchID, period)
	if err != nil {
		return nil, err
	}
	s.cacheSet(ctx, key, rows)
	return rows, nil
}

func (s *StaffAnalyticsService) waiterPerformance(ctx context.Context, branchID int64, period string) ([]WaiterPerformanceRow, error) {
	from, to := periodWindow(period)

	byStaff := map[int64]*WaiterPerformanceRow{}
	row := func(id int64, name, role string) *WaiterPerformanceRow {
		if r, ok := byStaff[id]; ok {
			return r
		}
		r := &WaiterPerformanceRow{StaffID: id, StaffName: name, StaffRole: role}
		byStaff[id] = r
		return r
	}

	orderActivity, err := s.repos.GetStaffOrderActivity(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, a := range orderActivity {
		r := row(a.StaffID, a.StaffName, string(a.StaffRole))
		r.OrdersAssisted = a.OrdersTouched
		r.OrdersServed = a.OrdersServed
		r.SessionsHandled = a.SessionsTouched
		r.TablesServed = a.TablesTouched
	}

	assistance, err := s.repos.GetStaffAssistanceStats(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, a := range assistance {
		r := row(a.StaffID, a.StaffName, string(a.StaffRole))
		r.AssistanceAccepted = a.AcknowledgedCount
		r.AssistanceResolved = a.ResolvedCount
		r.AvgResponseSeconds = a.AvgResponseSeconds
	}

	settlements, err := s.repos.GetStaffSettlementStats(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, p := range settlements {
		r := row(p.StaffID, p.StaffName, string(p.StaffRole))
		r.PaymentsSettled = p.PaymentsSettled
		r.AvgSettlementSeconds = p.AvgSettlementSeconds
		// "Sessions handled" approximates the union of sessions touched across
		// sources as the max (the sources cannot be union-distinct cheaply).
		if p.SessionsSettled > r.SessionsHandled {
			r.SessionsHandled = p.SessionsSettled
		}
	}

	logins, err := s.repos.GetStaffLoginStats(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, l := range logins {
		r := row(l.StaffID, l.StaffName, string(l.StaffRole))
		r.LoginCount = l.LoginCount
		r.ActiveSeconds = l.ActiveSeconds
	}

	out := make([]WaiterPerformanceRow, 0, len(byStaff))
	for _, r := range byStaff {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StaffName < out[j].StaffName })
	return out, nil
}

// GetKitchenPerformance returns per-staff kitchen metrics for the branch.
func (s *StaffAnalyticsService) GetKitchenPerformance(ctx context.Context, branchID int64, period string) ([]KitchenPerformanceRow, error) {
	if err := s.checkAccess(ctx, branchID); err != nil {
		return nil, err
	}
	key := cacheKey(branchID, period, "staff-kitchen")
	var cached []KitchenPerformanceRow
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	rows, err := s.kitchenPerformance(ctx, branchID, period)
	if err != nil {
		return nil, err
	}
	s.cacheSet(ctx, key, rows)
	return rows, nil
}

func (s *StaffAnalyticsService) kitchenPerformance(ctx context.Context, branchID int64, period string) ([]KitchenPerformanceRow, error) {
	from, to := periodWindow(period)

	byStaff := map[int64]*KitchenPerformanceRow{}
	completed, err := s.repos.GetKitchenPerformance(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, k := range completed {
		byStaff[k.StaffID] = &KitchenPerformanceRow{
			StaffID:         k.StaffID,
			StaffName:       k.StaffName,
			OrdersCompleted: k.OrdersCompleted,
			AvgPrepSeconds:  k.AvgPrepSeconds,
		}
	}
	peaks, err := s.repos.GetKitchenPeakThroughput(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, p := range peaks {
		r, ok := byStaff[p.StaffID]
		if !ok {
			r = &KitchenPerformanceRow{StaffID: p.StaffID, StaffName: p.StaffName}
			byStaff[p.StaffID] = r
		}
		r.PeakOrdersPerHour = p.PeakOrdersPerHour
	}

	out := make([]KitchenPerformanceRow, 0, len(byStaff))
	for _, r := range byStaff {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrdersCompleted > out[j].OrdersCompleted })
	return out, nil
}

// GetStaffSummary returns the per-staff daily activity summary for the branch.
func (s *StaffAnalyticsService) GetStaffSummary(ctx context.Context, branchID int64, period string) ([]StaffDailyActivityRow, error) {
	if err := s.checkAccess(ctx, branchID); err != nil {
		return nil, err
	}
	key := cacheKey(branchID, period, "staff-summary")
	var cached []StaffDailyActivityRow
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	rows, err := s.staffSummary(ctx, branchID, period)
	if err != nil {
		return nil, err
	}
	s.cacheSet(ctx, key, rows)
	return rows, nil
}

func (s *StaffAnalyticsService) staffSummary(ctx context.Context, branchID int64, period string) ([]StaffDailyActivityRow, error) {
	from, to := periodWindow(period)
	rows, err := s.repos.GetStaffDailyActivity(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]StaffDailyActivityRow, 0, len(rows))
	for _, r := range rows {
		day := ""
		if r.Day.Valid {
			day = r.Day.Time.UTC().Format("2006-01-02")
		}
		out = append(out, StaffDailyActivityRow{
			StaffID:       r.StaffID,
			StaffName:     r.StaffName,
			StaffRole:     string(r.StaffRole),
			Day:           day,
			EventCount:    r.EventCount,
			LoginCount:    r.LoginCount,
			ActiveSeconds: r.ActiveSeconds,
		})
	}
	return out, nil
}

// GetBranchPerformanceForPlatform bundles all three views for the platform
// operator read. NOT entitlement-gated: like the other /platform/analytics
// surfaces this is operator observability, independent of what the tenant has
// purchased; tenant-facing access stays gated.
func (s *StaffAnalyticsService) GetBranchPerformanceForPlatform(ctx context.Context, branchID int64, period string) (BranchStaffPerformance, error) {
	key := cacheKey(branchID, period, "staff-platform")
	var cached BranchStaffPerformance
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	waiters, err := s.waiterPerformance(ctx, branchID, period)
	if err != nil {
		return BranchStaffPerformance{}, err
	}
	kitchen, err := s.kitchenPerformance(ctx, branchID, period)
	if err != nil {
		return BranchStaffPerformance{}, err
	}
	summary, err := s.staffSummary(ctx, branchID, period)
	if err != nil {
		return BranchStaffPerformance{}, err
	}
	report := BranchStaffPerformance{
		BranchID: branchID,
		Period:   period,
		Waiters:  waiters,
		Kitchen:  kitchen,
		Summary:  summary,
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

func (s *StaffAnalyticsService) cacheGet(ctx context.Context, key string, dst any) bool {
	if s.cache == nil {
		return false
	}
	hit, _ := s.cache.Get(ctx, key, dst)
	return hit
}

func (s *StaffAnalyticsService) cacheSet(ctx context.Context, key string, v any) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Set(ctx, key, v, analyticsCacheTTL)
}

// IsStaffAnalyticsDisabled returns true if the error is the staff-analytics gate.
func IsStaffAnalyticsDisabled(err error) bool {
	return errors.Is(err, domain.ErrStaffAnalyticsDisabled)
}
