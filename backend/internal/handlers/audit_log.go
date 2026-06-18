package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditLogHandler struct {
	repos *repository.Repos
	authz *authz.Authorizer
	audit *audit.Writer
}

func NewAuditLogHandler(repos *repository.Repos, authorizer *authz.Authorizer, auditWriter *audit.Writer) *AuditLogHandler {
	return &AuditLogHandler{repos: repos, authz: authorizer, audit: auditWriter}
}

// GET /branches/:id/audit — owner/manager only; staff session must belong to this branch.
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
	actor, ok := staffActorForRequest(c, h.repos, sess)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, h.audit, actor, authz.ActionAuditReadBranch, authz.BranchResource(branchID, actor.Scope.OrganizationID)) {
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
	if h.audit != nil {
		h.audit.Record(c.Request.Context(), audit.AuditEvent{
			OrganizationID: actor.Scope.OrganizationID,
			BranchID:       branchID,
			ResourceType:   audit.ResourceAuditLog,
			Action:         audit.ActionAuditRead,
			ActorType:      audit.ActorTypeStaff,
			ActorID:        audit.IDStr(sess.StaffID),
			RiskLevel:      audit.RiskLow,
			Result:         audit.ResultSuccess,
		})
	}
}

// GET /orgs/:org_id/audit — active organization owners/admins only.
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
	member, err := h.repos.GetOrganizationMembershipForStaff(c.Request.Context(), orgID, sess.StaffID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			respondError(c, http.StatusForbidden, CodeForbidden, "organization access denied")
			return
		}
		respondInternalError(c)
		return
	}
	if member.Role != "owner" && member.Role != "admin" {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	params := sqlc.ListAuditLogForOrganizationParams{
		OrganizationID: pgtype.Int8{Int64: orgID, Valid: true},
		BranchID:       optionalInt8Param(c, "branch_id"),
		Action:         optionalTextParam(c, "action"),
		Result:         optionalTextParam(c, "result"),
		Source:         optionalTextParam(c, "source"),
		FromTime:       optionalTimestampParam(c, "from"),
		ToTime:         optionalTimestampParam(c, "to"),
	}

	logs, err := h.repos.ListAuditLogForOrganization(c.Request.Context(), params)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"audit_log": logs})
	if h.audit != nil {
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
