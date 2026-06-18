package repository_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/rs/zerolog"
)

func TestOrganizationCompatibilityQueries(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool, "organizations")

	fixtures := testutil.SeedFixtures(t, pool)
	repos := repository.New(pool, zerolog.Nop())
	ctx := context.Background()

	restaurant, err := repos.GetRestaurantBySlug(ctx, fixtures.RestaurantSlug)
	if err != nil {
		t.Fatalf("get restaurant by slug: %v", err)
	}
	if restaurant.OrganizationID != fixtures.OrganizationID {
		t.Fatalf("restaurant organization_id = %d, want %d", restaurant.OrganizationID, fixtures.OrganizationID)
	}

	orgByBranch, err := repos.GetOrganizationByBranchID(ctx, fixtures.BranchID)
	if err != nil {
		t.Fatalf("get organization by branch: %v", err)
	}
	if orgByBranch.ID != fixtures.OrganizationID {
		t.Fatalf("branch organization_id = %d, want %d", orgByBranch.ID, fixtures.OrganizationID)
	}

	branches, err := repos.ListBranchesForOrganization(ctx, fixtures.OrganizationID)
	if err != nil {
		t.Fatalf("list branches for organization: %v", err)
	}
	if len(branches) != 1 || branches[0].ID != fixtures.BranchID {
		t.Fatalf("branches = %+v, want exactly branch %d", branches, fixtures.BranchID)
	}
}

func TestOrganizationMembershipForStaff(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool, "organizations")

	fixtures := testutil.SeedFixtures(t, pool)
	repos := repository.New(pool, zerolog.Nop())
	ctx := context.Background()

	var staffID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO staff (branch_id, name, role, pin_hash, staff_code)
		VALUES ($1, 'Owner Test', 'owner', 'hash', 'OWNER01')
		RETURNING id
	`, fixtures.BranchID).Scan(&staffID); err != nil {
		t.Fatalf("insert staff: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO organization_members (organization_id, staff_id, role, status)
		VALUES ($1, $2, 'owner', 'active')
	`, fixtures.OrganizationID, staffID); err != nil {
		t.Fatalf("insert organization member: %v", err)
	}

	member, err := repos.GetOrganizationMembershipForStaff(ctx, fixtures.OrganizationID, staffID)
	if err != nil {
		t.Fatalf("get organization membership: %v", err)
	}
	if member.Role != "owner" {
		t.Fatalf("member role = %q, want owner", member.Role)
	}
}
