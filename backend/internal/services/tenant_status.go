package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/domain"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
)

// statusActive is the only organization/branch lifecycle status that serves
// guests. The column also carries "suspended" and "archived" (platform
// lifecycle, see handlers/platform_lifecycle.go); both stop new guest entry.
const statusActive = "active"

// tenantStatusCacheTTL bounds how long a lifecycle decision is reused. Every QR
// scan, session create and join consults this gate, so the uncached path (two
// row reads) must not run per request. A platform suspend/activate calls
// Invalidate, so the TTL is only the multi-instance backstop — the same shape
// as featureGateCacheTTL in feature_gate.go.
const tenantStatusCacheTTL = 60 * time.Second

// tenantStatus is the cached (organization, branch) lifecycle pair for a branch.
// Field names are terse because this is a hot Redis value, not an API payload.
type tenantStatus struct {
	OrganizationID     int64  `json:"o"`
	OrganizationStatus string `json:"os"`
	BranchStatus       string `json:"bs"`
}

// TenantStatusGate answers "may this branch admit a new guest right now?".
//
// Organization and branch statuses are two distinct gates that happen to share
// a vocabulary (active|suspended|archived):
//
//   - organization status is the billing/compliance state of the whole tenant.
//     Suspending it stops every branch the org owns.
//   - branch status is the operational state of one location. A branch can be
//     suspended (seasonal close, fit-out) while its organization is healthy.
//
// Neither is table status: sqlc.TableStatus is an occupancy enum
// (available|occupied|reserved) describing whether a table is in use, not
// whether it is permitted to serve. It is deliberately NOT consulted here —
// CreateSession already enforces occupancy through the one-active-session
// constraint, and treating "occupied" as a suspension would break every join.
//
// Suspension is a billing and compliance action, not an emergency stop: this
// gate governs ENTRY only (create, join, QR resolve). Sessions already in
// progress are never consulted and finish normally.
type TenantStatusGate struct {
	repos *repository.Repos
	cache *redisPkg.Cache
}

func NewTenantStatusGate(repos *repository.Repos, cache *redisPkg.Cache) *TenantStatusGate {
	return &TenantStatusGate{repos: repos, cache: cache}
}

// EnsureBranchAdmitsGuests returns nil when both the branch and its owning
// organization are active. Otherwise it returns domain.ErrOrganizationSuspended
// or domain.ErrBranchSuspended — the organization is checked first, because it
// is the broader statement about the tenant.
//
// A lookup failure is returned as-is: this gate never fails open.
func (g *TenantStatusGate) EnsureBranchAdmitsGuests(ctx context.Context, branchID int64) error {
	status, err := g.resolve(ctx, branchID)
	if err != nil {
		return err
	}
	if status.OrganizationStatus != statusActive {
		return domain.ErrOrganizationSuspended
	}
	if status.BranchStatus != statusActive {
		return domain.ErrBranchSuspended
	}
	return nil
}

func (g *TenantStatusGate) resolve(ctx context.Context, branchID int64) (tenantStatus, error) {
	cacheKey := fmt.Sprintf("tenantstatus:branch:%d", branchID)
	if g.cache != nil {
		var cached tenantStatus
		if hit, err := g.cache.Get(ctx, cacheKey, &cached); err == nil && hit {
			return cached, nil
		}
	}

	branch, err := g.repos.GetBranchByID(ctx, branchID)
	if err != nil {
		return tenantStatus{}, err
	}
	org, err := g.repos.GetOrganizationByID(ctx, branch.OrganizationID)
	if err != nil {
		return tenantStatus{}, err
	}
	status := tenantStatus{
		OrganizationID:     org.ID,
		OrganizationStatus: org.Status,
		BranchStatus:       branch.Status,
	}
	if g.cache != nil {
		_ = g.cache.Set(ctx, cacheKey, status, tenantStatusCacheTTL)
	}
	return status, nil
}

// Invalidate clears every cached lifecycle decision. Called when a platform
// operator suspends or activates an organization or branch so the change takes
// effect immediately rather than after tenantStatusCacheTTL.
//
// An organization flip fans out to every branch it owns, and those branch ids
// aren't known at the call site, so a full flush of the small tenantstatus
// keyspace is the correct, simplest choice — the same reasoning as
// FeatureGate.Invalidate. Operator lifecycle mutations are rare.
func (g *TenantStatusGate) Invalidate(ctx context.Context) {
	if g.cache == nil {
		return
	}
	_ = g.cache.DeleteByPattern(ctx, "tenantstatus:*")
}
