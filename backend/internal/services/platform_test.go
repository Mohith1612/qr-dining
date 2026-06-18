package services

import "testing"

func TestPlatformHasRole(t *testing.T) {
	session := PlatformSession{Roles: []string{PlatformRoleSupportAdmin}}
	if !PlatformHasRole(session, PlatformRoleSupportAdmin) {
		t.Fatal("support_admin should satisfy support_admin")
	}
	if PlatformHasRole(session, PlatformRoleBillingAdmin) {
		t.Fatal("support_admin should not satisfy billing_admin")
	}

	super := PlatformSession{Roles: []string{PlatformRoleSuperAdmin}}
	if !PlatformHasRole(super, PlatformRoleReadOnlyAuditor) {
		t.Fatal("super_admin should satisfy every platform role gate")
	}
}
