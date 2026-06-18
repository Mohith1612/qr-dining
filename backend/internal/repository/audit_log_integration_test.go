package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/audit"
	dbPkg "github.com/Mohith1612/qr-dining/internal/db"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// openAuditTestPool opens a pool, runs migrations, and truncates audit_log.
// It does NOT register pool.Close via t.Cleanup to avoid the stdlib-DB hang from RunMigrations.
// The pool is garbage-collected when the test binary exits.
func openAuditTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	if err := dbPkg.RunMigrations(pool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE audit_log CASCADE"); err != nil {
		t.Fatalf("truncate audit_log: %v", err)
	}
	return pool
}

func TestAuditLog_Insert(t *testing.T) {
	pool := openAuditTestPool(t)
	repos := repository.New(pool, zerolog.Nop())
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	w.Record(ctx, audit.AuditEvent{
		BranchID:     42,
		ResourceType: audit.ResourceBranch,
		ResourceID:   "42",
		Action:       audit.ActionBranchSettingsUpdate,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      "1",
		RiskLevel:    audit.RiskLow,
		Result:       audit.ResultSuccess,
	})

	logs, err := repos.ListAuditLogForBranch(ctx, sqlc.ListAuditLogForBranchParams{
		BranchID: pgtype.Int8{Int64: 42, Valid: true},
	})
	if err != nil {
		t.Fatalf("list audit log: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log row, got %d", len(logs))
	}
	row := logs[0]
	if row.Action != audit.ActionBranchSettingsUpdate {
		t.Errorf("action: got %v, want %v", row.Action, audit.ActionBranchSettingsUpdate)
	}
	if row.ActorID != "1" {
		t.Errorf("actor_id: got %v, want 1", row.ActorID)
	}
}

func TestAuditLog_Immutable(t *testing.T) {
	pool := openAuditTestPool(t)
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	w.Record(ctx, audit.AuditEvent{
		BranchID:     99,
		ResourceType: audit.ResourceBranch,
		ResourceID:   "99",
		Action:       audit.ActionBranchSettingsUpdate,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      "2",
	})

	// The DB trigger trg_audit_log_immutable must reject any UPDATE.
	_, err := pool.Exec(ctx, `UPDATE audit_log SET actor_id = 'tampered' WHERE branch_id = 99`)
	if err == nil {
		t.Fatal("expected immutability trigger to reject UPDATE, got nil error")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Errorf("expected 'immutable' in error, got: %v", err)
	}
}

func TestAuditLog_OrgIsolation(t *testing.T) {
	pool := openAuditTestPool(t)
	repos := repository.New(pool, zerolog.Nop())
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 100,
		ResourceType:   audit.ResourceOrganization,
		ResourceID:     "100",
		Action:         audit.ActionBranchSettingsUpdate,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        "10",
	})
	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 200,
		ResourceType:   audit.ResourceOrganization,
		ResourceID:     "200",
		Action:         audit.ActionBranchSettingsUpdate,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        "20",
	})

	logs100, err := repos.ListAuditLogForOrganization(ctx, sqlc.ListAuditLogForOrganizationParams{
		OrganizationID: pgtype.Int8{Int64: 100, Valid: true},
	})
	if err != nil {
		t.Fatalf("list org 100: %v", err)
	}
	if len(logs100) != 1 {
		t.Fatalf("org 100: expected 1 row, got %d", len(logs100))
	}
	if logs100[0].ActorID != "10" {
		t.Errorf("org 100 actor_id: got %v, want 10", logs100[0].ActorID)
	}

	logs200, err := repos.ListAuditLogForOrganization(ctx, sqlc.ListAuditLogForOrganizationParams{
		OrganizationID: pgtype.Int8{Int64: 200, Valid: true},
	})
	if err != nil {
		t.Fatalf("list org 200: %v", err)
	}
	if len(logs200) != 1 {
		t.Fatalf("org 200: expected 1 row, got %d", len(logs200))
	}
	if logs200[0].ActorID != "20" {
		t.Errorf("org 200 actor_id: got %v, want 20", logs200[0].ActorID)
	}
}

func TestAuditLog_OrganizationSourceFilter(t *testing.T) {
	pool := openAuditTestPool(t)
	repos := repository.New(pool, zerolog.Nop())
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 300,
		ResourceType:   audit.ResourceOrganization,
		ResourceID:     "300",
		Action:         audit.ActionBranchSettingsUpdate,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        "30",
		Source:         audit.SourceWeb,
	})
	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 300,
		ResourceType:   audit.ResourceOrganization,
		ResourceID:     "300",
		Action:         audit.ActionBranchSettingsUpdate,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        "31",
		Source:         audit.SourceAPI,
	})

	logs, err := repos.ListAuditLogForOrganization(ctx, sqlc.ListAuditLogForOrganizationParams{
		OrganizationID: pgtype.Int8{Int64: 300, Valid: true},
		Source:         pgtype.Text{String: string(audit.SourceWeb), Valid: true},
	})
	if err != nil {
		t.Fatalf("list org source: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 web source row, got %d", len(logs))
	}
	if logs[0].Source != sqlc.AuditSourceType(audit.SourceWeb) {
		t.Fatalf("source = %s, want %s", logs[0].Source, audit.SourceWeb)
	}
}

func TestAuditLog_PlatformFiltersV2Rows(t *testing.T) {
	pool := openAuditTestPool(t)
	repos := repository.New(pool, zerolog.Nop())
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 400,
		BranchID:       401,
		ResourceType:   audit.ResourcePlatformSupportSession,
		ResourceID:     "1",
		Action:         audit.ActionPlatformSupportAccess,
		ActorType:      audit.ActorTypePlatformUser,
		ActorID:        "99",
		Result:         audit.ResultSuccess,
		Source:         audit.SourceWeb,
		RiskLevel:      audit.RiskCritical,
	})
	w.Record(ctx, audit.AuditEvent{
		OrganizationID: 400,
		BranchID:       402,
		ResourceType:   audit.ResourcePlatformSupportSession,
		ResourceID:     "2",
		Action:         audit.ActionPlatformSupportAccess,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        "100",
		Result:         audit.ResultDenied,
		Source:         audit.SourceAPI,
		RiskLevel:      audit.RiskHigh,
	})

	logs, err := repos.ListAuditLogPlatform(ctx, sqlc.ListAuditLogPlatformParams{
		OrganizationID: pgtype.Int8{Int64: 400, Valid: true},
		ActorType:      pgtype.Text{String: string(audit.ActorTypePlatformUser), Valid: true},
		Result:         pgtype.Text{String: string(audit.ResultSuccess), Valid: true},
		Source:         pgtype.Text{String: string(audit.SourceWeb), Valid: true},
	})
	if err != nil {
		t.Fatalf("list platform audit v2: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 platform audit row, got %d", len(logs))
	}
	if logs[0].ActorType != sqlc.AuditActorType(audit.ActorTypePlatformUser) {
		t.Fatalf("actor_type = %s, want %s", logs[0].ActorType, audit.ActorTypePlatformUser)
	}
	if logs[0].Source != sqlc.AuditSourceType(audit.SourceWeb) || logs[0].Result != sqlc.AuditResultType(audit.ResultSuccess) {
		t.Fatalf("unexpected source/result: %s/%s", logs[0].Source, logs[0].Result)
	}
}

func TestAuditLog_RedactionInWriter(t *testing.T) {
	pool := openAuditTestPool(t)
	repos := repository.New(pool, zerolog.Nop())
	q := sqlc.New(pool)
	w := audit.NewWriter(q, true, zerolog.Nop(), nil)
	ctx := context.Background()

	before, _ := json.Marshal(map[string]any{
		"pin":  "1234",
		"role": "staff",
	})

	w.Record(ctx, audit.AuditEvent{
		BranchID:     77,
		ResourceType: audit.ResourceStaff,
		ResourceID:   "5",
		Action:       audit.ActionStaffPINReset,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      "5",
		Before:       before,
	})

	logs, err := repos.ListAuditLogForBranch(ctx, sqlc.ListAuditLogForBranchParams{
		BranchID: pgtype.Int8{Int64: 77, Valid: true},
	})
	if err != nil {
		t.Fatalf("list audit log: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 row, got %d", len(logs))
	}

	var stored map[string]any
	if err := json.Unmarshal(logs[0].BeforeJson, &stored); err != nil {
		t.Fatalf("unmarshal before_json: %v", err)
	}
	if stored["pin"] != "[REDACTED]" {
		t.Errorf("pin: got %v, want [REDACTED]", stored["pin"])
	}
	if stored["role"] != "staff" {
		t.Errorf("role: got %v, want staff", stored["role"])
	}
}
