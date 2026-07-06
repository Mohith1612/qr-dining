package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestPlatformAnalytics(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())
	svc := services.NewPlatformAnalyticsService(repos, nil) // nil cache: tolerated

	// ── Reports run against the live schema without error (empty data ok) ──
	usage, err := svc.GetUsage(ctx, nil, "daily")
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if usage.Period != "daily" {
		t.Fatalf("period = %q", usage.Period)
	}
	rev, err := svc.GetRevenue(ctx, nil, "monthly")
	if err != nil {
		t.Fatalf("GetRevenue: %v", err)
	}
	if rev.GMV != "0" {
		t.Errorf("GMV on empty data = %q, want 0", rev.GMV)
	}

	// ── Seed authz-denial audit rows for this org, assert health counts them ──
	writer := audit.NewWriter(sqlc.New(pool), true, zerolog.Nop(), nil)
	for i := 0; i < 2; i++ {
		writer.Record(ctx, audit.AuditEvent{
			OrganizationID: f.OrganizationID,
			BranchID:       f.BranchID,
			ResourceType:   "test_resource",
			ResourceID:     "1",
			Action:         "authz.denied",
			Result:         audit.ResultDenied,
			ActorType:      audit.ActorTypeStaff,
			ActorID:        "1",
		})
	}

	health, err := svc.GetHealth(ctx, &f.OrganizationID, "daily")
	if err != nil {
		t.Fatalf("GetHealth: %v", err)
	}
	var denials int64
	for _, d := range health.AuthzDenialsPerDay {
		denials += d.Count
	}
	if denials != 2 {
		t.Errorf("authz denials for org = %d, want 2", denials)
	}

	// ── Org filter isolates: a different org sees zero of our denials ──
	otherOrg := f.OrganizationID + 99999
	otherHealth, err := svc.GetHealth(ctx, &otherOrg, "daily")
	if err != nil {
		t.Fatalf("GetHealth(other): %v", err)
	}
	var otherDenials int64
	for _, d := range otherHealth.AuthzDenialsPerDay {
		otherDenials += d.Count
	}
	if otherDenials != 0 {
		t.Errorf("authz denials for unrelated org = %d, want 0", otherDenials)
	}
}
