package authz

import (
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

type Decision struct {
	Allowed       bool
	Reason        string
	RequiredScope Scope
	ActorScope    Scope
	ResourceScope Scope
	AuditHint     string
	// ScopeViolation marks a denial caused by tenant scope (the actor's branch or
	// organization does not own the resource) rather than by role policy. Cross-branch
	// and cross-organization access is never legitimate traffic, so callers enforce
	// these denials unconditionally instead of deferring to the rollout flag.
	ScopeViolation bool
}

type Authorizer struct {
	enforce bool
}

// NewAuthorizer constructs an authorizer in shadow mode: denials are recorded
// but Authorize() still reports the would-be decision so callers can choose to
// log-and-allow instead of blocking. This is the safe default during the
// rollout window in which legacy callers may not yet supply complete scope
// information.
func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

// NewEnforcingAuthorizer constructs an authorizer whose Enforce() reports true.
// Wired from the AUTHZ_CENTRAL_POLICY_ENFORCE feature flag so the cutover
// between shadow and strict mode is a one-line config change.
func NewEnforcingAuthorizer(enforce bool) *Authorizer {
	return &Authorizer{enforce: enforce}
}

// Enforce reports whether denials should block requests. When false, callers
// should record the would-be denial (metric + audit) but allow the request to
// proceed.
func (a *Authorizer) Enforce() bool { return a.enforce }

func (a *Authorizer) Authorize(actor Actor, action Action, resource Resource) Decision {
	decision := Decision{
		ActorScope:    actor.Scope,
		ResourceScope: resource.Scope,
		RequiredScope: resource.Scope,
		AuditHint:     fmt.Sprintf("%s:%s", action, resource.Type),
	}

	if actor.Type != ActorTypeStaff {
		decision.Reason = "unsupported actor type"
		return decision
	}

	if requiresSameBranch(action) && !actor.Scope.SameBranch(resource.Scope) {
		decision.Reason = "actor branch does not match resource branch"
		decision.ScopeViolation = true
		return decision
	}
	if requiresSameOrganization(action) && !actor.Scope.SameOrganization(resource.Scope) {
		decision.Reason = "actor organization does not match resource organization"
		decision.ScopeViolation = true
		return decision
	}

	if !roleAllowed(actor.Role, action) {
		decision.Reason = "role is not allowed for action"
		return decision
	}

	decision.Allowed = true
	decision.Reason = "allowed"
	return decision
}

func requiresSameBranch(action Action) bool {
	switch action {
	case ActionOrderStatusUpdate,
		ActionOrderMarkServed,
		ActionAssistanceAck,
		ActionAssistanceResolve,
		ActionMenuItemUpdate,
		ActionMenuItemToggleAvailable,
		ActionMenuCategoryUpdate,
		ActionMenuModifierUpdate,
		ActionPromoCreate,
		ActionPromoDeactivate,
		ActionPaymentSettleStaff,
		ActionStaffCreate,
		ActionStaffUpdateRole,
		ActionStaffDeactivate,
		ActionStaffPinUpdate,
		ActionStaffPinReset,
		ActionStaffListRead,
		ActionBranchRead,
		ActionBranchUpdateSettings,
		ActionAuditReadBranch:
		return true
	default:
		return false
	}
}

func requiresSameOrganization(action Action) bool {
	switch action {
	case ActionOrganizationRead, ActionOrganizationUpdate, ActionCustomerHistory, ActionCustomerDelete:
		return true
	default:
		return false
	}
}

func roleAllowed(role sqlc.StaffRole, action Action) bool {
	switch action {
	case ActionOrderStatusUpdate, ActionAssistanceAck, ActionAssistanceResolve:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter || role == sqlc.StaffRoleKitchen
	case ActionOrderMarkServed:
		// Serving is front-of-house: kitchen prepares to "ready", waiters serve.
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter
	case ActionMenuItemUpdate, ActionMenuItemToggleAvailable, ActionMenuCategoryUpdate, ActionMenuModifierUpdate, ActionPromoCreate, ActionPromoDeactivate, ActionCustomerDelete:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	case ActionCustomerHistory:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter || role == sqlc.StaffRoleKitchen
	case ActionStaffCreate, ActionStaffUpdateRole, ActionStaffDeactivate:
		return role == sqlc.StaffRoleOwner
	case ActionStaffPinUpdate:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter || role == sqlc.StaffRoleKitchen
	case ActionStaffPinReset, ActionStaffListRead:
		// Manager/owner can list staff and reset PINs (the forgotten-PIN path).
		// The handler additionally restricts which targets a manager may reset.
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	case ActionBranchRead:
		return role != ""
	case ActionAuditReadBranch:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	case ActionPaymentSettleStaff:
		// Waiters physically collect cash/card and confirm settlement.
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter
	case ActionBranchUpdateSettings, ActionOrganizationRead, ActionOrganizationUpdate:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	default:
		return false
	}
}
