package authz

import (
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
)

func TestAuthorizeSameBranchOperationalRoles(t *testing.T) {
	a := NewAuthorizer()
	resource := OrderResource(newUUID(t), 10, newUUID(t), 1)
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager, sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		decision := a.Authorize(staff(role, 10, 1), ActionOrderStatusUpdate, resource)
		if !decision.Allowed {
			t.Fatalf("role %s denied: %s", role, decision.Reason)
		}
	}
}

func TestAuthorizeMarkServedExcludesKitchen(t *testing.T) {
	a := NewAuthorizer()
	resource := OrderResource(newUUID(t), 10, newUUID(t), 1)
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager, sqlc.StaffRoleWaiter} {
		if d := a.Authorize(staff(role, 10, 1), ActionOrderMarkServed, resource); !d.Allowed {
			t.Fatalf("role %s denied mark-served: %s", role, d.Reason)
		}
	}
	if d := a.Authorize(staff(sqlc.StaffRoleKitchen, 10, 1), ActionOrderMarkServed, resource); d.Allowed {
		t.Fatal("kitchen unexpectedly allowed to mark order served")
	}
}

func TestAuthorizePaymentSettlementAllowsWaiter(t *testing.T) {
	a := NewAuthorizer()
	resource := Resource{Type: ResourceTypePayment, ID: "1", Scope: Scope{BranchID: 10, OrganizationID: 1}}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager, sqlc.StaffRoleWaiter} {
		if d := a.Authorize(staff(role, 10, 1), ActionPaymentSettleStaff, resource); !d.Allowed {
			t.Fatalf("role %s denied payment settlement: %s", role, d.Reason)
		}
	}
	if d := a.Authorize(staff(sqlc.StaffRoleKitchen, 10, 1), ActionPaymentSettleStaff, resource); d.Allowed {
		t.Fatal("kitchen unexpectedly allowed payment settlement")
	}
}

func TestAuthorizeDeniesCrossBranch(t *testing.T) {
	a := NewAuthorizer()
	decision := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionMenuItemUpdate, MenuItemResource(1, 10, 1))
	if decision.Allowed {
		t.Fatal("expected cross-branch decision to be denied")
	}
}

func TestAuthorizeDeniesCrossBranchOrderStatusUpdate(t *testing.T) {
	a := NewAuthorizer()
	resource := OrderResource(newUUID(t), 10, newUUID(t), 1)
	decision := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionOrderStatusUpdate, resource)
	if decision.Allowed {
		t.Fatal("expected staff from another branch to be denied order status update")
	}
}

func TestAuthorizeDeniesCrossBranchPaymentSettlement(t *testing.T) {
	a := NewAuthorizer()
	resource := Resource{Type: ResourceTypePayment, ID: "1", Scope: Scope{BranchID: 10, OrganizationID: 1}}
	decision := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionPaymentSettleStaff, resource)
	if decision.Allowed {
		t.Fatal("expected staff from another branch to be denied payment settlement")
	}
}

func TestAuthorizeMenuRoles(t *testing.T) {
	a := NewAuthorizer()
	resource := MenuItemResource(1, 10, 1)
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager} {
		if d := a.Authorize(staff(role, 10, 1), ActionMenuItemUpdate, resource); !d.Allowed {
			t.Fatalf("role %s denied: %s", role, d.Reason)
		}
	}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		if d := a.Authorize(staff(role, 10, 1), ActionMenuItemUpdate, resource); d.Allowed {
			t.Fatalf("role %s unexpectedly allowed", role)
		}
	}
}

func TestAuthorizeCustomerScope(t *testing.T) {
	a := NewAuthorizer()
	customer := CustomerResource(1, 100)
	if d := a.Authorize(staff(sqlc.StaffRoleWaiter, 10, 100), ActionCustomerHistory, customer); !d.Allowed {
		t.Fatalf("waiter same restaurant history denied: %s", d.Reason)
	}
	if d := a.Authorize(staff(sqlc.StaffRoleWaiter, 10, 100), ActionCustomerDelete, customer); d.Allowed {
		t.Fatal("waiter unexpectedly allowed to delete customer")
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 101), ActionCustomerHistory, customer); d.Allowed {
		t.Fatal("owner from another restaurant unexpectedly allowed customer history")
	}
}

func TestAuthorizeStaffDeactivationRequiresOwnerAndTargetBranch(t *testing.T) {
	a := NewAuthorizer()
	target := StaffResource(99, 10, 1)
	if d := a.Authorize(staff(sqlc.StaffRoleManager, 10, 1), ActionStaffDeactivate, target); d.Allowed {
		t.Fatal("manager unexpectedly allowed staff deactivation")
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionStaffDeactivate, target); d.Allowed {
		t.Fatal("owner from another branch unexpectedly allowed staff deactivation")
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 1), ActionStaffDeactivate, target); !d.Allowed {
		t.Fatalf("owner same branch denied: %s", d.Reason)
	}
}

func TestAuthorizeAuditReadBranchRequiresManagerOrOwner(t *testing.T) {
	a := NewAuthorizer()
	resource := BranchResource(10, 1)

	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager} {
		if d := a.Authorize(staff(role, 10, 1), ActionAuditReadBranch, resource); !d.Allowed {
			t.Fatalf("role %s denied audit read: %s", role, d.Reason)
		}
	}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		if d := a.Authorize(staff(role, 10, 1), ActionAuditReadBranch, resource); d.Allowed {
			t.Fatalf("role %s unexpectedly allowed audit read", role)
		}
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionAuditReadBranch, resource); d.Allowed {
		t.Fatal("cross-branch owner unexpectedly allowed audit read")
	}
}

// R3 governance-boundary regression locks (Decisions A/C): staff creation and
// branch-settings updates are branch-scoped owner/manager operations, and
// organization actions are org-scoped — none of which an actor from a different
// branch or organization may perform.

func TestAuthorizeStaffCreateRequiresOwnerAndSameBranch(t *testing.T) {
	a := NewAuthorizer()
	target := StaffResource(0, 10, 1)
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 1), ActionStaffCreate, target); !d.Allowed {
		t.Fatalf("owner same branch denied staff create: %s", d.Reason)
	}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleManager, sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		if d := a.Authorize(staff(role, 10, 1), ActionStaffCreate, target); d.Allowed {
			t.Fatalf("role %s unexpectedly allowed staff create", role)
		}
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionStaffCreate, target); d.Allowed {
		t.Fatal("owner from another branch unexpectedly allowed staff create")
	}
}

func TestAuthorizeBranchUpdateSettingsRolesAndBranch(t *testing.T) {
	a := NewAuthorizer()
	resource := BranchResource(10, 1)
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleOwner, sqlc.StaffRoleManager} {
		if d := a.Authorize(staff(role, 10, 1), ActionBranchUpdateSettings, resource); !d.Allowed {
			t.Fatalf("role %s denied branch update: %s", role, d.Reason)
		}
	}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		if d := a.Authorize(staff(role, 10, 1), ActionBranchUpdateSettings, resource); d.Allowed {
			t.Fatalf("role %s unexpectedly allowed branch update", role)
		}
	}
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionBranchUpdateSettings, resource); d.Allowed {
		t.Fatal("owner from another branch unexpectedly allowed branch update")
	}
}

func TestAuthorizeOrganizationScopeAndRoles(t *testing.T) {
	a := NewAuthorizer()
	resource := OrganizationResource(1)
	for _, action := range []Action{ActionOrganizationRead, ActionOrganizationUpdate} {
		if d := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 1), action, resource); !d.Allowed {
			t.Fatalf("owner same org denied %s: %s", action, d.Reason)
		}
		if d := a.Authorize(staff(sqlc.StaffRoleManager, 10, 1), action, resource); !d.Allowed {
			t.Fatalf("manager same org denied %s: %s", action, d.Reason)
		}
	}
	// Cross-org denied even for owner (org_mismatch), and operational roles never allowed.
	if d := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 2), ActionOrganizationUpdate, resource); d.Allowed {
		t.Fatal("owner from another org unexpectedly allowed organization update")
	}
	for _, role := range []sqlc.StaffRole{sqlc.StaffRoleWaiter, sqlc.StaffRoleKitchen} {
		if d := a.Authorize(staff(role, 10, 1), ActionOrganizationUpdate, resource); d.Allowed {
			t.Fatalf("role %s unexpectedly allowed organization update", role)
		}
	}
}

// TestScopeViolationIsSetOnlyForTenantScopeDenials pins the flag that makes tenant
// isolation independent of AUTHZ_CENTRAL_POLICY_ENFORCE. requireAuthorized enforces a
// denial unconditionally when ScopeViolation is true, so mislabelling a role denial as a
// scope violation would silently pull it out of the R3 shadow window, and failing to
// label a real scope denial would reopen the cross-tenant bypass this test exists to
// prevent.
func TestScopeViolationIsSetOnlyForTenantScopeDenials(t *testing.T) {
	a := NewAuthorizer()

	// Cross-branch: same org, different branch.
	crossBranch := a.Authorize(staff(sqlc.StaffRoleOwner, 11, 1), ActionMenuItemUpdate, MenuItemResource(1, 10, 1))
	if crossBranch.Allowed || !crossBranch.ScopeViolation {
		t.Fatalf("cross-branch: allowed=%v scope_violation=%v reason=%q; want denied scope violation",
			crossBranch.Allowed, crossBranch.ScopeViolation, crossBranch.Reason)
	}

	// Cross-organization on an org-scoped action.
	crossOrg := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 2), ActionOrganizationUpdate, OrganizationResource(1))
	if crossOrg.Allowed || !crossOrg.ScopeViolation {
		t.Fatalf("cross-org: allowed=%v scope_violation=%v reason=%q; want denied scope violation",
			crossOrg.Allowed, crossOrg.ScopeViolation, crossOrg.Reason)
	}

	// Role denial in the actor's own branch stays shadow-gated (not a scope violation).
	roleDenied := a.Authorize(staff(sqlc.StaffRoleWaiter, 10, 1), ActionMenuItemUpdate, MenuItemResource(1, 10, 1))
	if roleDenied.Allowed || roleDenied.ScopeViolation {
		t.Fatalf("role denial: allowed=%v scope_violation=%v reason=%q; want denied WITHOUT scope violation",
			roleDenied.Allowed, roleDenied.ScopeViolation, roleDenied.Reason)
	}

	// A permitted request carries no violation marker.
	allowed := a.Authorize(staff(sqlc.StaffRoleOwner, 10, 1), ActionMenuItemUpdate, MenuItemResource(1, 10, 1))
	if !allowed.Allowed || allowed.ScopeViolation {
		t.Fatalf("allowed: allowed=%v scope_violation=%v; want allowed without scope violation",
			allowed.Allowed, allowed.ScopeViolation)
	}
}

// TestScopeViolationFailsClosedOnZeroScope guards Scope.SameBranch/SameOrganization's
// zero-value behaviour: a resource or actor with an unresolved scope must deny, not
// coincidentally match another zero.
func TestScopeViolationFailsClosedOnZeroScope(t *testing.T) {
	a := NewAuthorizer()
	for name, actor := range map[string]Actor{
		"zero actor branch": staff(sqlc.StaffRoleOwner, 0, 1),
		"zero actor org":    staff(sqlc.StaffRoleOwner, 0, 0),
	} {
		d := a.Authorize(actor, ActionMenuItemUpdate, MenuItemResource(1, 0, 0))
		if d.Allowed || !d.ScopeViolation {
			t.Fatalf("%s: allowed=%v scope_violation=%v; want denied scope violation", name, d.Allowed, d.ScopeViolation)
		}
	}
}

func staff(role sqlc.StaffRole, branchID, orgID int64) Actor {
	return Actor{Type: ActorTypeStaff, ID: 1, Role: role, Scope: Scope{BranchID: branchID, OrganizationID: orgID}}
}

func newUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
