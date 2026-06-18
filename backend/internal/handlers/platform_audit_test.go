package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAuditV2PlatformFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/platform/audit?organization_id=10&branch_id=20&actor_type=platform_user&result=success&source=web", nil)
	c.Request = req

	got, ok := auditV2PlatformFilters(c)
	if !ok {
		t.Fatal("filters rejected valid query")
	}
	if got.OrganizationID.Int64 != 10 || !got.OrganizationID.Valid {
		t.Fatalf("organization_id = %+v, want 10", got.OrganizationID)
	}
	if got.BranchID.Int64 != 20 || !got.BranchID.Valid {
		t.Fatalf("branch_id = %+v, want 20", got.BranchID)
	}
	if got.ActorType.String != "platform_user" || !got.ActorType.Valid {
		t.Fatalf("actor_type = %+v, want platform_user", got.ActorType)
	}
	if got.Result.String != "success" || !got.Result.Valid {
		t.Fatalf("result = %+v, want success", got.Result)
	}
	if got.Source.String != "web" || !got.Source.Valid {
		t.Fatalf("source = %+v, want web", got.Source)
	}
}

func TestAuditV2PlatformResponseUsesRawJSON(t *testing.T) {
	row := sqlc.AuditLog{
		ID:             1,
		OrganizationID: pgtype.Int8{Int64: 10, Valid: true},
		ResourceType:   "staff",
		ResourceID:     "5",
		Action:         "staff.pin.reset",
		Result:         sqlc.AuditResultTypeSuccess,
		ActorType:      sqlc.AuditActorTypeStaff,
		ActorID:        "2",
		ActorScopeJson: json.RawMessage(`{"branch_id":20}`),
		Source:         sqlc.AuditSourceTypeWeb,
		BeforeJson:     []byte(`{"pin":"[REDACTED]"}`),
		MetadataJson:   json.RawMessage(`{"reason":"test"}`),
		RiskLevel:      sqlc.AuditRiskLevelMedium,
	}

	body, err := json.Marshal(auditV2PlatformResponse(row))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	before, ok := got["before"].(map[string]any)
	if !ok {
		t.Fatalf("before encoded as %T, want JSON object", got["before"])
	}
	if before["pin"] != "[REDACTED]" {
		t.Fatalf("before.pin = %v, want [REDACTED]", before["pin"])
	}
	if got["organization_id"] != float64(10) {
		t.Fatalf("organization_id = %v, want 10", got["organization_id"])
	}
}
