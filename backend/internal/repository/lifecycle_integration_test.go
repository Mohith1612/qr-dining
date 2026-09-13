package repository_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestOrganizationAndBranchStatusLifecycle(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	ctx := context.Background()
	repos := repository.New(pool, zerolog.Nop())

	// Organizations default to active.
	org, err := repos.GetOrganizationByID(ctx, f.OrganizationID)
	if err != nil {
		t.Fatalf("get org: %v", err)
	}
	if org.Status != "active" {
		t.Fatalf("seed org status = %q, want active", org.Status)
	}

	// Suspend → active round-trip.
	suspended, err := repos.UpdateOrganizationStatus(ctx, f.OrganizationID, "suspended")
	if err != nil {
		t.Fatalf("suspend org: %v", err)
	}
	if suspended.Status != "suspended" {
		t.Fatalf("org status = %q, want suspended", suspended.Status)
	}
	reactivated, err := repos.UpdateOrganizationStatus(ctx, f.OrganizationID, "active")
	if err != nil {
		t.Fatalf("activate org: %v", err)
	}
	if reactivated.Status != "active" {
		t.Fatalf("org status = %q, want active", reactivated.Status)
	}

	// Branch suspend → active round-trip.
	branch, err := repos.UpdateBranchStatus(ctx, f.BranchID, "suspended")
	if err != nil {
		t.Fatalf("suspend branch: %v", err)
	}
	if branch.Status != "suspended" {
		t.Fatalf("branch status = %q, want suspended", branch.Status)
	}
	branch, err = repos.UpdateBranchStatus(ctx, f.BranchID, "active")
	if err != nil {
		t.Fatalf("activate branch: %v", err)
	}
	if branch.Status != "active" {
		t.Fatalf("branch status = %q, want active", branch.Status)
	}
}
