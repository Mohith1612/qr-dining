package services

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

// Flag resolution sources, surfaced for operator observability.
const (
	flagSourceBranchOverride = "branch_override"
	flagSourceOrgOverride    = "org_override"
	flagSourceGlobalOverride = "global_override"
	flagSourceDefault        = "default"
)

// FlagService resolves platform product feature flags with precedence
// branch > organization > global > catalog default. This is entirely separate
// from the env-driven strict-rollout flags in internal/config.
type FlagService struct {
	repos   *repository.Repos
	metrics *observability.Metrics
}

func NewFlagService(repos *repository.Repos, metrics *observability.Metrics) *FlagService {
	return &FlagService{repos: repos, metrics: metrics}
}

// FlagState is the resolved value of one flag for a scope.
type FlagState struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}

// resolveFlag applies precedence branch > org > global > default. A nil pointer
// means "no override at that level".
func resolveFlag(defaultEnabled bool, global, org, branch *bool) (bool, string) {
	switch {
	case branch != nil:
		return *branch, flagSourceBranchOverride
	case org != nil:
		return *org, flagSourceOrgOverride
	case global != nil:
		return *global, flagSourceGlobalOverride
	default:
		return defaultEnabled, flagSourceDefault
	}
}

// ResolveForBranch resolves every catalog flag for a branch (branch > org > global > default).
func (s *FlagService) ResolveForBranch(ctx context.Context, branchID int64) ([]FlagState, error) {
	branch, err := s.repos.GetBranchByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	catalog, err := s.repos.ListFeatureFlags(ctx)
	if err != nil {
		return nil, err
	}
	globals, err := s.globalMap(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.orgMap(ctx, branch.OrganizationID)
	if err != nil {
		return nil, err
	}
	branchOverrides := map[string]bool{}
	branchRows, err := s.repos.ListBranchFlagOverrides(ctx, branchID)
	if err != nil {
		return nil, err
	}
	for _, b := range branchRows {
		branchOverrides[b.FlagKey] = b.Enabled
	}

	out := make([]FlagState, len(catalog))
	for i, f := range catalog {
		enabled, source := resolveFlag(f.DefaultEnabled, ptrFromMap(globals, f.Key), ptrFromMap(orgs, f.Key), ptrFromMap(branchOverrides, f.Key))
		out[i] = FlagState{Key: f.Key, Enabled: enabled, Source: source}
	}
	s.recordResolution("branch")
	return out, nil
}

// ResolveForOrganization resolves every catalog flag for an org (org > global > default).
func (s *FlagService) ResolveForOrganization(ctx context.Context, orgID int64) ([]FlagState, error) {
	catalog, err := s.repos.ListFeatureFlags(ctx)
	if err != nil {
		return nil, err
	}
	globals, err := s.globalMap(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.orgMap(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]FlagState, len(catalog))
	for i, f := range catalog {
		enabled, source := resolveFlag(f.DefaultEnabled, ptrFromMap(globals, f.Key), ptrFromMap(orgs, f.Key), nil)
		out[i] = FlagState{Key: f.Key, Enabled: enabled, Source: source}
	}
	s.recordResolution("organization")
	return out, nil
}

func (s *FlagService) globalMap(ctx context.Context) (map[string]bool, error) {
	rows, err := s.repos.ListGlobalFlagOverrides(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(rows))
	for _, g := range rows {
		m[g.FlagKey] = g.Enabled
	}
	return m, nil
}

func (s *FlagService) orgMap(ctx context.Context, orgID int64) (map[string]bool, error) {
	rows, err := s.repos.ListOrganizationFlagOverrides(ctx, orgID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(rows))
	for _, o := range rows {
		m[o.FlagKey] = o.Enabled
	}
	return m, nil
}

func (s *FlagService) recordResolution(scope string) {
	if s.metrics == nil || s.metrics.FeatureFlagResolutionsTotal == nil {
		return
	}
	s.metrics.FeatureFlagResolutionsTotal.WithLabelValues(scope).Inc()
}

func ptrFromMap(m map[string]bool, key string) *bool {
	if v, ok := m[key]; ok {
		return &v
	}
	return nil
}
