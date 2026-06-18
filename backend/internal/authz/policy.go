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
}

type Authorizer struct{}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

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
		return decision
	}
	if requiresSameOrganization(action) && !actor.Scope.SameOrganization(resource.Scope) {
		decision.Reason = "actor organization does not match resource organization"
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
	case ActionMenuItemUpdate, ActionMenuItemToggleAvailable, ActionMenuCategoryUpdate, ActionMenuModifierUpdate, ActionPromoCreate, ActionPromoDeactivate, ActionCustomerDelete:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	case ActionCustomerHistory:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter || role == sqlc.StaffRoleKitchen
	case ActionStaffCreate, ActionStaffUpdateRole, ActionStaffDeactivate:
		return role == sqlc.StaffRoleOwner
	case ActionStaffPinUpdate:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager || role == sqlc.StaffRoleWaiter || role == sqlc.StaffRoleKitchen
	case ActionBranchRead:
		return role != ""
	case ActionAuditReadBranch:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	case ActionBranchUpdateSettings, ActionOrganizationRead, ActionOrganizationUpdate, ActionPaymentSettleStaff:
		return role == sqlc.StaffRoleOwner || role == sqlc.StaffRoleManager
	default:
		return false
	}
}
