package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
)

type TableHandler struct {
	repos *repository.Repos
	audit *audit.Writer
}

func NewTableHandler(repos *repository.Repos, auditWriter *audit.Writer) *TableHandler {
	return &TableHandler{repos: repos, audit: auditWriter}
}

type tableResponse struct {
	ID          int64  `json:"id"`
	BranchID    int64  `json:"branch_id"`
	Identifier  string `json:"identifier"`
	Capacity    int16  `json:"capacity"`
	QRCodeToken string `json:"qr_code_token"`
	Status      string `json:"status"`
}

func tableToResponse(t sqlc.Table) tableResponse {
	return tableResponse{
		ID:          t.ID,
		BranchID:    t.BranchID,
		Identifier:  t.Identifier,
		Capacity:    t.Capacity,
		QRCodeToken: t.QrCodeToken,
		Status:      string(t.Status),
	}
}

// GET /branches/:id/tables
func (h *TableHandler) ListTables(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	rows, err := h.repos.ListTablesForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	out := make([]tableResponse, len(rows))
	for i, row := range rows {
		out[i] = tableToResponse(row)
	}

	c.JSON(http.StatusOK, gin.H{"tables": out})
}

type createTableRequest struct {
	Identifier string `json:"identifier" binding:"required,min=1,max=50"`
	Capacity   int16  `json:"capacity"`
}

// POST /branches/:id/tables — owner/manager only
func (h *TableHandler) CreateTable(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	if sess.Role != sqlc.StaffRoleOwner && sess.Role != sqlc.StaffRoleManager {
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners and managers can create tables")
		return
	}

	var req createTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	if req.Capacity <= 0 {
		req.Capacity = 4
	}

	token, err := crypto.GenerateToken()
	if err != nil {
		respondInternalError(c)
		return
	}

	table, err := h.repos.CreateTable(c.Request.Context(), sqlc.CreateTableParams{
		BranchID:    branchID,
		Identifier:  req.Identifier,
		Capacity:    req.Capacity,
		QrCodeToken: token,
	})
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateTableIdentifier) {
			respondError(c, http.StatusConflict, CodeDuplicateTableIdentifier, err.Error())
			return
		}
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, tableToResponse(table))
}

// PATCH /tables/:id/qr-refresh — owner/manager only
func (h *TableHandler) RefreshQR(c *gin.Context) {
	tableID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid table id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if sess.Role != sqlc.StaffRoleOwner && sess.Role != sqlc.StaffRoleManager {
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners and managers can regenerate QR tokens")
		return
	}

	table, err := h.repos.GetTableByID(c.Request.Context(), tableID)
	if err != nil {
		if errors.Is(err, domain.ErrTableNotFound) {
			respondError(c, http.StatusNotFound, CodeTableNotFound, err.Error())
			return
		}
		respondInternalError(c)
		return
	}

	if table.BranchID != sess.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	if table.Status == sqlc.TableStatusOccupied {
		respondError(c, http.StatusConflict, CodeTableOccupied, "cannot regenerate QR token while table is occupied")
		return
	}

	newToken, err := crypto.GenerateToken()
	if err != nil {
		respondInternalError(c)
		return
	}

	updated, err := h.repos.RefreshTableQRToken(c.Request.Context(), tableID, newToken)
	if err != nil {
		respondInternalError(c)
		return
	}

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     updated.BranchID,
		ResourceType: audit.ResourceQRToken,
		ResourceID:   audit.IDStr(tableID),
		Action:       audit.ActionQRTokenRotate,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskMedium,
		Metadata:     map[string]any{"table_identifier": updated.Identifier},
	})
	c.JSON(http.StatusOK, tableToResponse(updated))
}
