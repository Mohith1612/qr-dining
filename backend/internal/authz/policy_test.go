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

func staff(role sqlc.StaffRole, branchID, orgID int64) Actor {
	return Actor{Type: ActorTypeStaff, ID: 1, Role: role, Scope: Scope{BranchID: branchID, OrganizationID: orgID}}
}

func newUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
