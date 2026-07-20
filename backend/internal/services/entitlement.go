package services

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// Entitlement catalog keys — mirror the seed in migration 000029.
const (
	EntitlementAnalyticsBasic    = "analytics.basic"
	EntitlementAnalyticsAdvanced = "analytics.advanced"
	EntitlementCustomTheme       = "custom.theme"
	EntitlementMultiBranch       = "multi_branch"
	EntitlementAdvancedAudit     = "advanced.audit"
	EntitlementSupportPriority   = "support.priority"
	EntitlementAPIAccess         = "api.access"

	EntitlementLimitBranches = "limit.branches"
	EntitlementLimitStaff    = "limit.staff"
	EntitlementLimitTables   = "limit.tables"

	// Seeded in migrations 000034 (staff analytics) and 000035 (loyalty).
	// Granted to no plan by default — resolve true only via an explicit
	// plan grant or org override.
	EntitlementStaffPerformanceAnalytics = "analytics.staff_performance"
	EntitlementLoyaltyEnabled            = "loyalty.enabled"
	EntitlementLoyaltyRedeem             = "loyalty.redeem"
	EntitlementLoyaltyManualAdjustment   = "loyalty.manual_adjustment"
)

// LimitUnlimited is the sentinel for "no ceiling".
const LimitUnlimited int64 = -1

// Resolution sources, surfaced for operator observability.
const (
	entSourceOrgAssignment    = "org_assignment"
	entSourceRestaurantBridge = "restaurant_bridge"
	entSourceFreeDefault      = "free_default"
)

// EntitlementService resolves organization-level entitlements. It is resolve-only:
// HasCapability/Limit compute the effective state and emit a shadow metric, but no
// operational path enforces these results in this phase.
type EntitlementService struct {
	repos   *repository.Repos
	subSvc  *SubscriptionService
	metrics *observability.Metrics
}

func NewEntitlementService(repos *repository.Repos, subSvc *SubscriptionService, metrics *observability.Metrics) *EntitlementService {
	return &EntitlementService{repos: repos, subSvc: subSvc, metrics: metrics}
}

// EffectiveEntitlements is the resolved entitlement state for an organization.
type EffectiveEntitlements struct {
	OrganizationID int64            `json:"organization_id"`
	PlanTier       string           `json:"plan_tier"`
	Capabilities   map[string]bool  `json:"capabilities"`
	Limits         map[string]int64 `json:"limits"` // -1 = unlimited
	Source         string           `json:"source"`
}

// DB-free inputs to mergeEntitlements so resolution is unit-testable.
type catalogEntitlement struct {
	Key  string
	Kind string
}

type baseEntitlement struct {
	Key     string
	Enabled bool
	Limit   *int64 // nil = unlimited
}

type overrideEntitlement struct {
	Key     string
	Enabled *bool  // nil = inherit
	Limit   *int64 // nil = inherit
}

// mergeEntitlements resolves capabilities and limits from a catalog, a base layer
// (plan defaults or the features_json bridge), and org-level overrides.
// Capabilities default false; limits default unlimited (-1) unless the base sets a value.
func mergeEntitlements(catalog []catalogEntitlement, base []baseEntitlement, overrides []overrideEntitlement) (map[string]bool, map[string]int64) {
	baseByKey := make(map[string]baseEntitlement, len(base))
	for _, b := range base {
		baseByKey[b.Key] = b
	}
	ovByKey := make(map[string]overrideEntitlement, len(overrides))
	for _, o := range overrides {
		ovByKey[o.Key] = o
	}

	caps := map[string]bool{}
	limits := map[string]int64{}
	for _, e := range catalog {
		switch e.Kind {
		case "capability":
			v := false
			if b, ok := baseByKey[e.Key]; ok {
				v = b.Enabled
			}
			if o, ok := ovByKey[e.Key]; ok && o.Enabled != nil {
				v = *o.Enabled
			}
			caps[e.Key] = v
		case "limit":
			v := LimitUnlimited
			if b, ok := baseByKey[e.Key]; ok && b.Limit != nil {
				v = *b.Limit
			}
			if o, ok := ovByKey[e.Key]; ok && o.Limit != nil {
				v = *o.Limit
			}
			limits[e.Key] = v
		}
	}
	return caps, limits
}

// ResolveForOrganization computes the effective entitlements for an organization.
// Precedence: org overrides > org plan assignment > restaurant-subscription bridge >
// free-tier default.
func (s *EntitlementService) ResolveForOrganization(ctx context.Context, orgID int64) (EffectiveEntitlements, error) {
	catalogRows, err := s.repos.ListEntitlementCatalog(ctx)
	if err != nil {
		return EffectiveEntitlements{}, err
	}
	catalog := make([]catalogEntitlement, len(catalogRows))
	for i, c := range catalogRows {
		catalog[i] = catalogEntitlement{Key: c.Key, Kind: c.Kind}
	}

	overrideRows, err := s.repos.ListOrganizationEntitlementOverrides(ctx, orgID)
	if err != nil {
		return EffectiveEntitlements{}, err
	}
	overrides := make([]overrideEntitlement, len(overrideRows))
	for i, o := range overrideRows {
		overrides[i] = overrideEntitlement{
			Key:     o.EntitlementKey,
			Enabled: boolPtrFromPg(o.Enabled),
			Limit:   int64PtrFromPg(o.LimitValue),
		}
	}

	var base []baseEntitlement
	tier := "free"
	source := entSourceFreeDefault

	assignment, err := s.repos.GetOrganizationPlanAssignment(ctx, orgID)
	switch {
	case err == nil:
		planRows, perr := s.repos.ListPlanEntitlements(ctx, assignment.PlanID)
		if perr != nil {
			return EffectiveEntitlements{}, perr
		}
		base = make([]baseEntitlement, len(planRows))
		for i, p := range planRows {
			base[i] = baseEntitlement{Key: p.EntitlementKey, Enabled: p.Enabled, Limit: int64PtrFromPg(p.LimitValue)}
		}
		if plan, perr := s.repos.GetPlanByID(ctx, assignment.PlanID); perr == nil {
			tier = string(plan.Tier)
		}
		source = entSourceOrgAssignment
	case errors.Is(err, domain.ErrOrgPlanNotAssigned):
		base, tier, source = s.bridgeFromRestaurant(ctx, orgID)
	default:
		return EffectiveEntitlements{}, err
	}

	caps, limits := mergeEntitlements(catalog, base, overrides)
	return EffectiveEntitlements{
		OrganizationID: orgID,
		PlanTier:       tier,
		Capabilities:   caps,
		Limits:         limits,
		Source:         source,
	}, nil
}

// bridgeFromRestaurant derives base entitlements from the org's restaurant
// subscription (org↔restaurant is 1:1 today). This keeps the new system a strict
// superset of existing behavior even when no plan_entitlements rows exist.
func (s *EntitlementService) bridgeFromRestaurant(ctx context.Context, orgID int64) ([]baseEntitlement, string, string) {
	freeDefault := repository.PlanFeatures{MaxBranches: 1, MaxTables: 10}

	restaurant, err := s.repos.GetRestaurantByOrganizationID(ctx, orgID)
	if err != nil {
		return bridgeFeatures(freeDefault), "free", entSourceFreeDefault
	}
	features, err := s.subSvc.GetPlanFeatures(ctx, restaurant.ID)
	if err != nil {
		return bridgeFeatures(freeDefault), "free", entSourceFreeDefault
	}
	tier := "free"
	source := entSourceFreeDefault
	if sub, serr := s.repos.GetSubscriptionByRestaurant(ctx, restaurant.ID); serr == nil {
		tier = string(sub.PlanTier)
		source = entSourceRestaurantBridge
	}
	return bridgeFeatures(features), tier, source
}

// bridgeFeatures maps the legacy features_json struct onto entitlement base rows.
func bridgeFeatures(f repository.PlanFeatures) []baseEntitlement {
	rows := []baseEntitlement{
		{Key: EntitlementLimitBranches, Enabled: true, Limit: limitFromInt(f.MaxBranches)},
		{Key: EntitlementLimitTables, Enabled: true, Limit: limitFromInt(f.MaxTables)},
	}
	if f.Analytics {
		rows = append(rows, baseEntitlement{Key: EntitlementAnalyticsBasic, Enabled: true})
	}
	if f.MultiBranch {
		rows = append(rows, baseEntitlement{Key: EntitlementMultiBranch, Enabled: true})
	}
	return rows
}

// HasCapability resolves whether an org has a capability and records a shadow
// evaluation metric. Never blocks the caller in this phase.
func (s *EntitlementService) HasCapability(ctx context.Context, orgID int64, key string) (bool, error) {
	eff, err := s.ResolveForOrganization(ctx, orgID)
	if err != nil {
		return false, err
	}
	allowed := eff.Capabilities[key]
	s.recordEvaluation(key, allowed)
	return allowed, nil
}

// Limit returns the effective numeric limit for a key (-1 = unlimited).
func (s *EntitlementService) Limit(ctx context.Context, orgID int64, key string) (int64, error) {
	eff, err := s.ResolveForOrganization(ctx, orgID)
	if err != nil {
		return 0, err
	}
	if v, ok := eff.Limits[key]; ok {
		return v, nil
	}
	return LimitUnlimited, nil
}

func (s *EntitlementService) recordEvaluation(capability string, allowed bool) {
	if s.metrics == nil || s.metrics.EntitlementEvaluationsTotal == nil {
		return
	}
	result := "deny"
	if allowed {
		result = "allow"
	}
	s.metrics.EntitlementEvaluationsTotal.WithLabelValues(capability, result).Inc()
}

func limitFromInt(v int) *int64 {
	if v < 0 {
		return nil // -1 = unlimited
	}
	x := int64(v)
	return &x
}

func boolPtrFromPg(b pgtype.Bool) *bool {
	if !b.Valid {
		return nil
	}
	v := b.Bool
	return &v
}

func int64PtrFromPg(n pgtype.Int8) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
