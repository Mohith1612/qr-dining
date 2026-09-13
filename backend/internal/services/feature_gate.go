package services

import (
	"context"
	"fmt"
	"time"

	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

// Platform feature-flag catalog keys consumed by gated features.
// Seeded (default_enabled = FALSE) in migrations 000034 and 000035.
const (
	FlagStaffPerformanceAnalytics = "staff_performance_analytics"
	FlagLoyalty                   = "loyalty"
)

// featureGateCacheTTL bounds how long a gate decision is reused. A platform
// flag/entitlement flip therefore propagates to consumers (including the
// payment-completion loyalty hook) within at most this window.
const featureGateCacheTTL = 60 * time.Second

// FeatureGate answers "is feature X enabled for this branch/org?" by combining
// the two governance systems: the org must hold the entitlement capability
// (plan grant or org override) AND the platform product flag must resolve
// enabled for the scope (branch > org > global > catalog default). Both
// default off, so a feature is inert until a platform operator deliberately
// grants the entitlement and enables the flag.
type FeatureGate struct {
	ent   *EntitlementService
	flags *FlagService
	repos *repository.Repos
	cache *redisPkg.Cache
}

func NewFeatureGate(ent *EntitlementService, flags *FlagService, repos *repository.Repos, cache *redisPkg.Cache) *FeatureGate {
	return &FeatureGate{ent: ent, flags: flags, repos: repos, cache: cache}
}

// gateDecision is the pure combination rule, split out for unit testing.
func gateDecision(hasCapability, flagEnabled bool) bool {
	return hasCapability && flagEnabled
}

// BranchEnabled resolves the gate for a branch scope.
func (g *FeatureGate) BranchEnabled(ctx context.Context, branchID int64, entKey, flagKey string) (bool, error) {
	cacheKey := fmt.Sprintf("featgate:branch:%d:%s:%s", branchID, entKey, flagKey)
	if hit, enabled := g.cacheGet(ctx, cacheKey); hit {
		return enabled, nil
	}
	branch, err := g.repos.GetBranchByID(ctx, branchID)
	if err != nil {
		return false, err
	}
	hasCap, err := g.ent.HasCapability(ctx, branch.OrganizationID, entKey)
	if err != nil {
		return false, err
	}
	flag, err := g.flags.ResolveOneForBranch(ctx, branchID, flagKey)
	if err != nil {
		return false, err
	}
	enabled := gateDecision(hasCap, flag.Enabled)
	g.cacheSet(ctx, cacheKey, enabled)
	return enabled, nil
}

// OrganizationEnabled resolves the gate for an org scope (no branch override layer).
func (g *FeatureGate) OrganizationEnabled(ctx context.Context, orgID int64, entKey, flagKey string) (bool, error) {
	cacheKey := fmt.Sprintf("featgate:org:%d:%s:%s", orgID, entKey, flagKey)
	if hit, enabled := g.cacheGet(ctx, cacheKey); hit {
		return enabled, nil
	}
	hasCap, err := g.ent.HasCapability(ctx, orgID, entKey)
	if err != nil {
		return false, err
	}
	flag, err := g.flags.ResolveOneForOrganization(ctx, orgID, flagKey)
	if err != nil {
		return false, err
	}
	enabled := gateDecision(hasCap, flag.Enabled)
	g.cacheSet(ctx, cacheKey, enabled)
	return enabled, nil
}

func (g *FeatureGate) cacheGet(ctx context.Context, key string) (hit, enabled bool) {
	if g.cache == nil {
		return false, false
	}
	var cached bool
	ok, err := g.cache.Get(ctx, key, &cached)
	if err != nil || !ok {
		return false, false
	}
	return true, cached
}

func (g *FeatureGate) cacheSet(ctx context.Context, key string, enabled bool) {
	if g.cache == nil {
		return
	}
	_ = g.cache.Set(ctx, key, enabled, featureGateCacheTTL)
}

// Invalidate clears every cached gate decision. Called when a platform operator
// changes a feature flag or entitlement so the change is visible immediately
// rather than only after featureGateCacheTTL lapses. A global/org flag change
// fans out to many branches whose exact keys aren't known here, so a full flush
// of the small featgate keyspace is the correct, simplest choice — operator
// mutations are rare and low-volume. The TTL remains a multi-instance backstop.
func (g *FeatureGate) Invalidate(ctx context.Context) {
	if g.cache == nil {
		return
	}
	_ = g.cache.DeleteByPattern(ctx, "featgate:*")
}
