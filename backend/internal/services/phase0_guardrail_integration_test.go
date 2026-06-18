//go:build integration

package services_test

import (
	"os"
	"testing"
)

func phase0GuardrailsEnabled(t testing.TB) {
	t.Helper()
	if os.Getenv("RUN_PHASE0_GUARDRAIL_TESTS") != "true" {
		t.Skip("pending Phase 0 guardrail: set RUN_PHASE0_GUARDRAIL_TESTS=true to convert this known-risk scaffold into an active test")
	}
}

func TestPhase0Guardrail_DuplicatePINAndInactiveStaffLogin(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected StaffService.Authenticate to reject ambiguous duplicate active PINs and inactive staff PIN matches")
}

func TestPhase0Guardrail_CrossBranchOrderStatusDenied(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected staff from branch A to be denied when updating an order owned by branch B")
}

func TestPhase0Guardrail_CrossBranchAssistanceMutationDenied(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected staff from branch A to be denied when acknowledging or resolving assistance owned by branch B")
}

func TestPhase0Guardrail_SpoofedOrderSessionBranchMismatchDenied(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected PlaceOrder to reject a session from branch A combined with client-supplied branch B and branch B menu items")
}

func TestPhase0Guardrail_UnauthenticatedWebhookRejected(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected payment webhook handler/service path to reject unsigned provider webhook payloads")
}

func TestPhase0Guardrail_StaleSessionCleanupReleasesTable(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected stale session cleanup to release the occupied table as part of abandoning the session")
}

func TestPhase0Guardrail_DuplicateActiveSessionRacePrevented(t *testing.T) {
	phase0GuardrailsEnabled(t)
	t.Fatal("expected concurrent CreateSession calls for one table to allow only one active session")
}
