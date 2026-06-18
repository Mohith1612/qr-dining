package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditLogHandler struct {
	repos *repository.Repos
	audit *audit.Writer
}

func NewAuditLogHandler(repos *repository.Repos, auditWriter *audit.Writer) *AuditLogHandler {
	return &AuditLogHandler{repos: repos, audit: auditWriter}
}

// GET /branches/:id/audit — branch staff only; staff session must belong to this branch.
func (h *AuditLogHandler) GetBranchAuditLog(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	params := sqlc.ListAuditLogForBranchParams{
		BranchID: pgtype.Int8{Int64: branchID, Valid: true},
		Action:   optionalTextParam(c, "action"),
		Result:   optionalTextParam(c, "result"),
		Source:   optionalTextParam(c, "source"),
		FromTime: optionalTimestampParam(c, "from"),
		ToTime:   optionalTimestampParam(c, "to"),
	}

	logs, err := h.repos.ListAuditLogForBranch(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"audit_log": logs})
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     branchID,
		ResourceType: audit.ResourceAuditLog,
		Action:       audit.ActionAuditRead,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskLow,
		Result:       audit.ResultSuccess,
	})
}

// GET /orgs/:org_id/audit — org staff only; staff session must belong to this org.
func (h *AuditLogHandler) GetOrgAuditLog(c *gin.Context) {
	orgID, err := strconv.ParseInt(c.Param("org_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid org id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if sess.OrganizationID != orgID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	params := sqlc.ListAuditLogForOrganizationParams{
		OrganizationID: pgtype.Int8{Int64: orgID, Valid: true},
		BranchID:       optionalInt8Param(c, "branch_id"),
		Action:         optionalTextParam(c, "action"),
		Result:         optionalTextParam(c, "result"),
		FromTime:       optionalTimestampParam(c, "from"),
		ToTime:         optionalTimestampParam(c, "to"),
	}

	logs, err := h.repos.ListAuditLogForOrganization(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"audit_log": logs})
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		OrganizationID: orgID,
		ResourceType:   audit.ResourceAuditLog,
		Action:         audit.ActionAuditRead,
		ActorType:      audit.ActorTypeStaff,
		ActorID:        audit.IDStr(sess.StaffID),
		RiskLevel:      audit.RiskLow,
		Result:         audit.ResultSuccess,
	})
}

func optionalTextParam(c *gin.Context, key string) pgtype.Text {
	v := c.Query(key)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func optionalInt8Param(c *gin.Context, key string) pgtype.Int8 {
	v := c.Query(key)
	if v == "" {
		return pgtype.Int8{}
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: n, Valid: true}
}

func optionalTimestampParam(c *gin.Context, key string) pgtype.Timestamptz {
	v := c.Query(key)
	if v == "" {
		return pgtype.Timestamptz{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
