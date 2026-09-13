package services

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

// EnforcementObservabilityService surfaces where enforcement WOULD bite if it were
// turned on — expiring/suspended subscriptions, over-limit tenants, and feature-flag
// override usage. It is strictly read-only and never gates any path; it mirrors the
// "observe before enforce" discipline (resolve -> observe -> enforce). On-demand
// Postgres aggregation, Redis-cached, reusing the platform-analytics shape.
type EnforcementObservabilityService struct {
	repos *repository.Repos
	ent   *EntitlementService
	cache *redis.Cache
}

func NewEnforcementObservabilityService(repos *repository.Repos, ent *EntitlementService, cache *redis.Cache) *EnforcementObservabilityService {
	return &EnforcementObservabilityService{repos: repos, ent: ent, cache: cache}
}

const enforcementObservabilityCacheTTL = 5 * time.Minute

// trialEndingWindow is how far ahead a trial must end to be flagged as "ending soon".
const trialEndingWindow = 7 * 24 * time.Hour

// ── Subscriptions ────────────────────────────────────────────────────────────

type SubscriptionAlert struct {
	OrganizationID int64      `json:"organization_id"`
	OrgCode        string     `json:"org_code"`
	OrgName        string     `json:"org_name"`
	Status         string     `json:"status"`
	PlanTier       string     `json:"plan_tier"`
	Reason         string     `json:"reason"` // suspended|expired|past_due|trial_expired|trial_ending
	TrialEndsAt    *time.Time `json:"trial_ends_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

type SubscriptionObservabilityReport struct {
	Total  int                 `json:"total"`
	Alerts []SubscriptionAlert `json:"alerts"`
}

// SubscriptionObservability buckets every org subscription into a "would be affected"
// alert list (suspended / expired / past_due / trial expired / trial ending soon).
func (s *EnforcementObservabilityService) SubscriptionObservability(ctx context.Context) (SubscriptionObservabilityReport, error) {
	const key = "enforce_obs:subscriptions"
	var cached SubscriptionObservabilityReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	rows, err := s.repos.ListSubscriptionsForObservability(ctx)
	if err != nil {
		return SubscriptionObservabilityReport{}, err
	}
	now := time.Now().UTC()
	report := SubscriptionObservabilityReport{Total: len(rows), Alerts: []SubscriptionAlert{}}
	for _, r := range rows {
		trial := timePtr(r.TrialEndsAt)
		expires := timePtr(r.ExpiresAt)
		reason := classifySubscription(r.Status, trial, expires, now)
		if reason == "" {
			continue
		}
		report.Alerts = append(report.Alerts, SubscriptionAlert{
			OrganizationID: r.OrganizationID, OrgCode: r.OrgCode, OrgName: r.OrgName,
			Status: r.Status, PlanTier: string(r.PlanTier), Reason: reason,
			TrialEndsAt: trial, ExpiresAt: expires,
		})
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

// classifySubscription returns the alert reason for a subscription, or "" if healthy.
// Pure + unit-testable.
func classifySubscription(status string, trialEndsAt, expiresAt *time.Time, now time.Time) string {
	switch status {
	case SubStatusSuspended:
		return "suspended"
	case SubStatusPastDue:
		return "past_due"
	case SubStatusExpired:
		return "expired"
	case SubStatusCancelled:
		return ""
	case SubStatusActive:
		if expiresAt != nil && expiresAt.Before(now) {
			return "expired" // lapsed but not yet swept
		}
	case SubStatusTrial:
		if trialEndsAt != nil {
			if trialEndsAt.Before(now) {
				return "trial_expired"
			}
			if trialEndsAt.Before(now.Add(trialEndingWindow)) {
				return "trial_ending"
			}
		}
	}
	return ""
}

// ── Entitlement limits ───────────────────────────────────────────────────────

type LimitBreach struct {
	OrganizationID int64  `json:"organization_id"`
	OrgCode        string `json:"org_code"`
	OrgName        string `json:"org_name"`
	Key            string `json:"key"`
	Limit          int64  `json:"limit"`
	Actual         int64  `json:"actual"`
}

type EntitlementObservabilityReport struct {
	Checked  int           `json:"checked"`
	Breaches []LimitBreach `json:"breaches"`
}

// EntitlementObservability compares each org's actual resource usage against its
// resolved entitlement limits and lists the over-limit cases.
func (s *EnforcementObservabilityService) EntitlementObservability(ctx context.Context) (EntitlementObservabilityReport, error) {
	const key = "enforce_obs:entitlements"
	var cached EntitlementObservabilityReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	counts, err := s.repos.ListOrgResourceCounts(ctx)
	if err != nil {
		return EntitlementObservabilityReport{}, err
	}
	report := EntitlementObservabilityReport{Checked: len(counts), Breaches: []LimitBreach{}}
	for _, c := range counts {
		eff, err := s.ent.ResolveForOrganization(ctx, c.OrganizationID)
		if err != nil {
			return EntitlementObservabilityReport{}, err
		}
		check := func(limitKey string, actual int64) {
			limit, ok := eff.Limits[limitKey]
			if !ok || limit == LimitUnlimited {
				return
			}
			if actual > limit {
				report.Breaches = append(report.Breaches, LimitBreach{
					OrganizationID: c.OrganizationID, OrgCode: c.Code, OrgName: c.Name,
					Key: limitKey, Limit: limit, Actual: actual,
				})
			}
		}
		check(EntitlementLimitBranches, c.BranchCount)
		check(EntitlementLimitStaff, c.StaffCount)
		check(EntitlementLimitTables, c.MaxTablesPerBranch) // limit.tables is per-branch
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

// ── Feature flags ────────────────────────────────────────────────────────────

type FlagOverrideStat struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Global int64  `json:"global"`
	Org    int64  `json:"org"`
	Branch int64  `json:"branch"`
	Total  int64  `json:"total"`
}

type FlagObservabilityReport struct {
	InUse    []FlagOverrideStat `json:"in_use"`
	Orphaned []string           `json:"orphaned"` // catalog flags with no overrides anywhere
}

// FlagObservability reports which feature flags have overrides in use (and at what
// scope) and which catalog flags are orphaned (defined but never overridden).
func (s *EnforcementObservabilityService) FlagObservability(ctx context.Context) (FlagObservabilityReport, error) {
	const key = "enforce_obs:flags"
	var cached FlagObservabilityReport
	if s.cacheGet(ctx, key, &cached) {
		return cached, nil
	}
	catalog, err := s.repos.ListFlagCatalogKeys(ctx)
	if err != nil {
		return FlagObservabilityReport{}, err
	}
	counts, err := s.repos.ListFlagOverrideCounts(ctx)
	if err != nil {
		return FlagObservabilityReport{}, err
	}
	stats := map[string]*FlagOverrideStat{}
	names := map[string]string{}
	for _, c := range catalog {
		names[c.Key] = c.Name
	}
	for _, oc := range counts {
		st := stats[oc.FlagKey]
		if st == nil {
			st = &FlagOverrideStat{Key: oc.FlagKey, Name: names[oc.FlagKey]}
			stats[oc.FlagKey] = st
		}
		switch oc.Scope {
		case "global":
			st.Global += oc.N
		case "organization":
			st.Org += oc.N
		case "branch":
			st.Branch += oc.N
		}
		st.Total += oc.N
	}
	report := FlagObservabilityReport{InUse: []FlagOverrideStat{}, Orphaned: []string{}}
	for _, c := range catalog {
		if st, ok := stats[c.Key]; ok && st.Total > 0 {
			report.InUse = append(report.InUse, *st)
		} else {
			report.Orphaned = append(report.Orphaned, c.Key)
		}
	}
	s.cacheSet(ctx, key, report)
	return report, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *EnforcementObservabilityService) cacheGet(ctx context.Context, key string, dst any) bool {
	if s.cache == nil {
		return false
	}
	hit, _ := s.cache.Get(ctx, key, dst)
	return hit
}

func (s *EnforcementObservabilityService) cacheSet(ctx context.Context, key string, v any) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Set(ctx, key, v, enforcementObservabilityCacheTTL)
}
