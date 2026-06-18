package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

func TestPlatformProvisioningRepositories(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	testutil.TruncateTables(t, pool, "platform_users", "organizations")

	repos := repository.New(pool, zerolog.Nop())
	ctx := context.Background()

	passwordHash, err := services.HashPlatformPassword("super-secret-password")
	if err != nil {
		t.Fatalf("hash platform password: %v", err)
	}
	user, err := repos.UpsertPlatformUser(ctx, sqlc.UpsertPlatformUserParams{
		Email:        "admin@example.test",
		DisplayName:  "Admin Test",
		PasswordHash: passwordHash,
		Status:       "active",
		MfaRequired:  false,
	})
	if err != nil {
		t.Fatalf("upsert platform user: %v", err)
	}
	if err := repos.AddPlatformUserRole(ctx, user.ID, services.PlatformRoleSuperAdmin); err != nil {
		t.Fatalf("add role: %v", err)
	}
	roles, err := repos.ListPlatformRolesForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 1 || roles[0] != services.PlatformRoleSuperAdmin {
		t.Fatalf("roles = %v, want super_admin", roles)
	}

	org, err := repos.CreatePlatformOrganization(ctx, sqlc.CreatePlatformOrganizationParams{
		Code:                "phase4-test",
		Name:                "Phase 4 Test",
		LegalName:           "Phase 4 Test LLC",
		PrimaryContactEmail: "owner@example.test",
		SettingsJson:        json.RawMessage(`{"theme":"modern-minimal"}`),
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	restaurant, err := repos.CreatePlatformRestaurant(ctx, sqlc.CreatePlatformRestaurantParams{
		Name:           "Phase 4 Restaurant",
		Slug:           "phase4-test",
		SettingsJson:   json.RawMessage(`{}`),
		OrganizationID: org.ID,
	})
	if err != nil {
		t.Fatalf("create restaurant: %v", err)
	}
	branch, err := repos.CreatePlatformBranch(ctx, sqlc.CreatePlatformBranchParams{
		RestaurantID:   restaurant.ID,
		OrganizationID: org.ID,
		Name:           "Main",
		Address:        "1 Test St",
		Timezone:       "UTC",
		BranchCode:     "P4-MAIN",
		OrderPrefix:    "P4",
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	if err := repos.CreateOrganizationBranchMembership(ctx, org.ID, branch.ID); err != nil {
		t.Fatalf("create branch membership: %v", err)
	}

	support, err := repos.CreatePlatformSupportSession(ctx, sqlc.CreatePlatformSupportSessionParams{
		PlatformUserID: user.ID,
		OrganizationID: org.ID,
		BranchID:       pgtype.Int8{Int64: branch.ID, Valid: true},
		Reason:         "Investigating customer-reported issue",
		StartsAt:       time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create support session: %v", err)
	}
	if support.OrganizationID != org.ID || !support.BranchID.Valid || support.BranchID.Int64 != branch.ID {
		t.Fatalf("support session scope = org %d branch %+v, want org %d branch %d", support.OrganizationID, support.BranchID, org.ID, branch.ID)
	}

	repos.LogPlatformAudit(ctx, repository.PlatformAuditParams{
		PlatformUserID:   user.ID,
		Action:           "platform.test",
		TargetType:       "organization",
		TargetID:         "phase4-test",
		OrganizationID:   org.ID,
		BranchID:         branch.ID,
		SupportSessionID: support.ID,
		RequestID:        "test-request",
		Payload:          map[string]any{"ok": true},
	})
	audit, err := repos.ListPlatformAuditLog(ctx, sqlc.ListPlatformAuditLogParams{
		PlatformUserID: pgtype.Int8{Int64: user.ID, Valid: true},
		Action:         pgtype.Text{String: "platform.test", Valid: true},
	})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(audit) != 1 || audit[0].SupportSessionID.Int64 != support.ID {
		t.Fatalf("audit rows = %+v, want one row for support session %d", audit, support.ID)
	}
}
