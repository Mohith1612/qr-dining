package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestBillingLifecycle(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool,
		"subscription_payments",
		"subscription_invoices",
		"organization_subscriptions",
		"organization_billing_profiles",
		"organization_entitlement_overrides",
		"organization_plan_assignments",
		"plan_entitlements",
		"restaurant_subscriptions",
		"subscription_plans",
	)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()

	repos := repository.New(pool, zerolog.Nop())
	billing := services.NewBillingService(repos)
	ent := services.NewEntitlementService(repos, services.NewSubscriptionService(repos), observability.NewMetrics())

	// Two plans so we can test plan changes flowing into the resolver.
	var standardID, premiumID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
		 VALUES ('Standard','standard',1499,'{"max_branches":3,"max_tables":-1,"analytics":true}'::jsonb) RETURNING id`).Scan(&standardID); err != nil {
		t.Fatalf("insert standard: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO subscription_plans (name, tier, price_monthly, features_json)
		 VALUES ('Premium','premium',2999,'{}'::jsonb) RETURNING id`).Scan(&premiumID); err != nil {
		t.Fatalf("insert premium: %v", err)
	}

	expires := time.Now().Add(30 * 24 * time.Hour).UTC()

	// ── Activate (create) ────────────────────────────────────────────────────
	sub, err := billing.Activate(ctx, f.OrganizationID, &standardID, &expires, 0)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if sub.Status != services.SubStatusActive {
		t.Fatalf("status = %q, want active", sub.Status)
	}
	if !sub.StartedAt.Valid || !sub.ExpiresAt.Valid {
		t.Error("expected started_at and expires_at set on activate")
	}

	// Activation must sync organization_plan_assignments so the resolver tracks the plan.
	eff, err := ent.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve after activate: %v", err)
	}
	if eff.Source != "org_assignment" || eff.PlanTier != "standard" {
		t.Fatalf("after activate: source=%q tier=%q, want org_assignment/standard", eff.Source, eff.PlanTier)
	}

	// ── Suspend ──────────────────────────────────────────────────────────────
	sub, err = billing.Suspend(ctx, f.OrganizationID, 0)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if sub.Status != services.SubStatusSuspended || !sub.SuspendedAt.Valid {
		t.Fatalf("after suspend: status=%q suspended_at.valid=%v", sub.Status, sub.SuspendedAt.Valid)
	}

	// ── Activate again resumes a suspended subscription (clears suspension) ───
	sub, err = billing.Activate(ctx, f.OrganizationID, nil, &expires, 0)
	if err != nil {
		t.Fatalf("resume via activate: %v", err)
	}
	if sub.Status != services.SubStatusActive || sub.SuspendedAt.Valid {
		t.Fatalf("after resume: status=%q suspended=%v", sub.Status, sub.SuspendedAt.Valid)
	}

	// ── Renew extends an active subscription ─────────────────────────────────
	newExpiry := time.Now().Add(60 * 24 * time.Hour).UTC()
	sub, err = billing.Renew(ctx, f.OrganizationID, &newExpiry, 0)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if sub.Status != services.SubStatusActive || !sub.RenewedAt.Valid {
		t.Fatalf("after renew: status=%q renewed=%v", sub.Status, sub.RenewedAt.Valid)
	}

	// ── Change plan (syncs resolver to premium) ──────────────────────────────
	if _, err = billing.ChangePlan(ctx, f.OrganizationID, premiumID, 0); err != nil {
		t.Fatalf("change plan: %v", err)
	}
	eff, err = ent.ResolveForOrganization(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("resolve after change plan: %v", err)
	}
	if eff.PlanTier != "premium" {
		t.Fatalf("after change plan: tier=%q, want premium", eff.PlanTier)
	}

	// ── Cancel + invalid follow-on transition ────────────────────────────────
	sub, err = billing.Cancel(ctx, f.OrganizationID, "non-payment", 0)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if sub.Status != services.SubStatusCancelled || !sub.CancelledAt.Valid || sub.CancellationReason != "non-payment" {
		t.Fatalf("after cancel: status=%q reason=%q", sub.Status, sub.CancellationReason)
	}
	if _, err = billing.Suspend(ctx, f.OrganizationID, 0); !errors.Is(err, domain.ErrInvalidSubscriptionTransition) {
		t.Fatalf("suspend after cancel: err=%v, want ErrInvalidSubscriptionTransition", err)
	}

	// ── Billing profile upsert + read ────────────────────────────────────────
	if _, err = billing.UpsertBillingProfile(ctx, repository.BillingProfileParams{
		OrganizationID: f.OrganizationID,
		BusinessName:   "Maison Saffron Pvt Ltd",
		GstNumber:      "29ABCDE1234F1Z5",
		BillingEmail:   "billing@example.com",
	}); err != nil {
		t.Fatalf("upsert billing profile: %v", err)
	}
	profile, found, err := billing.GetBillingProfile(ctx, f.OrganizationID)
	if err != nil || !found || profile.GstNumber != "29ABCDE1234F1Z5" {
		t.Fatalf("get billing profile: found=%v gst=%q err=%v", found, profile.GstNumber, err)
	}

	// ── Invoice lifecycle ────────────────────────────────────────────────────
	inv, err := billing.CreateInvoice(ctx, f.OrganizationID, "1499.00", "INR", &expires, "May subscription", 0)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if inv.Status != "draft" || inv.InvoiceNumber == "" {
		t.Fatalf("draft invoice: status=%q number=%q", inv.Status, inv.InvoiceNumber)
	}
	inv, err = billing.TransitionInvoice(ctx, f.OrganizationID, inv.ID, "issue")
	if err != nil || inv.Status != "issued" || !inv.IssueDate.Valid {
		t.Fatalf("issue invoice: status=%q err=%v", inv.Status, err)
	}
	inv, err = billing.TransitionInvoice(ctx, f.OrganizationID, inv.ID, "mark_paid")
	if err != nil || inv.Status != "paid" || !inv.PaidAt.Valid {
		t.Fatalf("mark paid: status=%q err=%v", inv.Status, err)
	}
	// Paid is terminal — issuing again is invalid.
	if _, err = billing.TransitionInvoice(ctx, f.OrganizationID, inv.ID, "issue"); !errors.Is(err, domain.ErrInvalidInvoiceTransition) {
		t.Fatalf("issue paid invoice: err=%v, want ErrInvalidInvoiceTransition", err)
	}
	paidInvoiceID := inv.ID

	// A second invoice can be cancelled from draft.
	inv2, err := billing.CreateInvoice(ctx, f.OrganizationID, "100.00", "", nil, "", 0)
	if err != nil {
		t.Fatalf("create invoice 2: %v", err)
	}
	if inv2.InvoiceNumber == inv.InvoiceNumber {
		t.Fatal("invoice numbers must be unique")
	}
	if _, err = billing.TransitionInvoice(ctx, f.OrganizationID, inv2.ID, "cancel"); err != nil {
		t.Fatalf("cancel invoice 2: %v", err)
	}

	// ── Manual payment + validation ──────────────────────────────────────────
	pay, err := billing.RecordPayment(ctx, f.OrganizationID, "upi", "1499.00", "INR", "UTR123", "first month", nil, &paidInvoiceID, 0)
	if err != nil {
		t.Fatalf("record payment: %v", err)
	}
	if pay.Method != "upi" || pay.ProviderType != "manual" {
		t.Fatalf("payment: method=%q provider=%q", pay.Method, pay.ProviderType)
	}
	if _, err = billing.RecordPayment(ctx, f.OrganizationID, "crypto", "1.00", "INR", "", "", nil, nil, 0); !services.IsInvalidPaymentMethod(err) {
		t.Fatalf("invalid method: err=%v, want IsInvalidPaymentMethod", err)
	}

	// ── Org isolation ────────────────────────────────────────────────────────
	var orgB int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (code, name, settings_json) VALUES ($1,'Org B','{}') RETURNING id`,
		"org-b-"+timeSuffix()).Scan(&orgB); err != nil {
		t.Fatalf("insert org B: %v", err)
	}
	// Trial via extend-trial create path.
	trialEnd := time.Now().Add(14 * 24 * time.Hour).UTC()
	subB, err := billing.ExtendTrial(ctx, orgB, &standardID, trialEnd, 0)
	if err != nil || subB.Status != services.SubStatusTrial || !subB.TrialEndsAt.Valid {
		t.Fatalf("extend trial create: status=%q err=%v", subB.Status, err)
	}
	// Org A's paid invoice must not be reachable through org B.
	if _, err = billing.GetInvoice(ctx, orgB, paidInvoiceID); !errors.Is(err, domain.ErrInvoiceNotFound) {
		t.Fatalf("cross-org invoice read: err=%v, want ErrInvoiceNotFound", err)
	}
	// Recording a payment under org B against org A's invoice must be rejected.
	if _, err = billing.RecordPayment(ctx, orgB, "cash", "1.00", "INR", "", "", nil, &paidInvoiceID, 0); !errors.Is(err, domain.ErrInvoiceNotFound) {
		t.Fatalf("cross-org payment: err=%v, want ErrInvoiceNotFound", err)
	}
	// Listings are scoped per org.
	invA, _ := billing.ListInvoices(ctx, f.OrganizationID)
	invB, _ := billing.ListInvoices(ctx, orgB)
	if len(invA) != 2 {
		t.Fatalf("org A invoices = %d, want 2", len(invA))
	}
	if len(invB) != 0 {
		t.Fatalf("org B invoices = %d, want 0", len(invB))
	}
}

func timeSuffix() string {
	return time.Now().Format("150405.000000")
}
