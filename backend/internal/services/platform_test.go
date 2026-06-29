package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/rs/zerolog"
)

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

// TestBeginMFAEnrollmentRequiresEncryptionKey guards PT-03: when
// MFA_ENCRYPTION_KEY is unset, enrollment must surface a classified
// operational error (mapped to 503 by the handler), never a generic internal
// error. mfaKey() fails before any repository access, so a nil-repo service is
// sufficient for this path.
func TestBeginMFAEnrollmentRequiresEncryptionKey(t *testing.T) {
	svc := NewPlatformService(nil, nil, zerolog.Nop())
	if _, err := svc.BeginMFAEnrollment(context.Background(), 1); !errors.Is(err, domain.ErrMFANotConfigured) {
		t.Fatalf("expected domain.ErrMFANotConfigured, got %v", err)
	}
}
