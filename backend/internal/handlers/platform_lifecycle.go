package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
)

// Organization & branch lifecycle (platform-governed). These flip the existing
// status columns (active|suspended|archived) and emit a platform audit row. No
// operational path reads these statuses yet — the flip is inert until a future
// enforcement phase, preserving rollout/soak safety.

// SuspendOrganization sets an organization's status to suspended.
// POST /platform/organizations/:org_id/suspend — super_admin.
func (h *PlatformHandler) SuspendOrganization(c *gin.Context) {
	h.setOrganizationStatus(c, "suspended", "platform.organizations.suspend")
}

// ActivateOrganization sets an organization's status to active.
// POST /platform/organizations/:org_id/activate — super_admin.
func (h *PlatformHandler) ActivateOrganization(c *gin.Context) {
	h.setOrganizationStatus(c, "active", "platform.organizations.activate")
}

func (h *PlatformHandler) setOrganizationStatus(c *gin.Context, status, action string) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	orgID, ok := parseInt64Param(c, "org_id", "invalid organization id")
	if !ok {
		return
	}
	if _, err := h.repos.GetOrganizationByID(c.Request.Context(), orgID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "organization not found")
		return
	}
	org, err := h.repos.UpdateOrganizationStatus(c.Request.Context(), orgID, status)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, action, "organization", strconv.FormatInt(org.ID, 10), org.ID, 0, 0, gin.H{"status": status})
	c.JSON(http.StatusOK, organizationResponse(org))
}

// SuspendBranch sets a branch's status to suspended.
// POST /platform/branches/:branch_id/suspend — super_admin.
func (h *PlatformHandler) SuspendBranch(c *gin.Context) {
	h.setBranchStatus(c, "suspended", "platform.branches.suspend")
}

// ActivateBranch sets a branch's status to active.
// POST /platform/branches/:branch_id/activate — super_admin.
func (h *PlatformHandler) ActivateBranch(c *gin.Context) {
	h.setBranchStatus(c, "active", "platform.branches.activate")
}

func (h *PlatformHandler) setBranchStatus(c *gin.Context, status, action string) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	if _, err := h.repos.GetBranchByID(c.Request.Context(), branchID); err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	branch, err := h.repos.UpdateBranchStatus(c.Request.Context(), branchID, status)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, action, "branch", strconv.FormatInt(branch.ID, 10), branch.OrganizationID, branch.ID, 0, gin.H{"status": status})
	c.JSON(http.StatusOK, platformBranchResponse(branch))
}

// Batch table provisioning for tenant onboarding. The branch-creation endpoint already
// accepts initial_tables; this lets the operator add tables to an existing branch (the
// onboarding wizard's dedicated "generate tables" step). Additive; QR tokens are minted
// the same way as everywhere else (crypto.GenerateToken).

const maxBatchTables = 200

type batchCreateTablesRequest struct {
	Count       int      `json:"count"`
	Capacity    int16    `json:"capacity"`
	Identifiers []string `json:"identifiers"`
}

// BatchCreateBranchTables creates N tables on a branch in one transaction. Provide either
// an explicit `identifiers` list or a `count` (auto-named T<n>, offset past existing tables).
// POST /platform/branches/:branch_id/tables — super_admin.
func (h *PlatformHandler) BatchCreateBranchTables(c *gin.Context) {
	session, ok := h.requirePlatformRole(c)
	if !ok {
		return
	}
	branchID, ok := parseInt64Param(c, "branch_id", "invalid branch id")
	if !ok {
		return
	}
	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeTenantNotFound, "branch not found")
		return
	}
	var req batchCreateTablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	// Resolve the identifier list: explicit list wins; otherwise auto-generate from count,
	// offset past the branch's existing tables to avoid collisions.
	identifiers := make([]string, 0, len(req.Identifiers))
	for _, id := range req.Identifiers {
		if t := strings.TrimSpace(id); t != "" {
			identifiers = append(identifiers, t)
		}
	}
	if len(identifiers) == 0 {
		if req.Count <= 0 {
			respondValidationError(c, "provide identifiers or a positive count")
			return
		}
		existing, err := h.repos.ListTablesForBranch(c.Request.Context(), branchID)
		if err != nil {
			respondInternalError(c)
			return
		}
		offset := len(existing)
		for i := 1; i <= req.Count; i++ {
			identifiers = append(identifiers, fmt.Sprintf("T%d", offset+i))
		}
	}
	if len(identifiers) > maxBatchTables {
		respondValidationError(c, fmt.Sprintf("cannot create more than %d tables at once", maxBatchTables))
		return
	}
	capacity := req.Capacity
	if capacity <= 0 {
		capacity = 4
	}

	var tables []sqlc.Table
	err = h.repos.WithTx(c.Request.Context(), func(tx *repository.Repos) error {
		for _, ident := range identifiers {
			token, terr := crypto.GenerateToken()
			if terr != nil {
				return terr
			}
			table, terr := tx.CreateTable(c.Request.Context(), sqlc.CreateTableParams{
				BranchID:    branchID,
				Identifier:  ident,
				Capacity:    capacity,
				QrCodeToken: token,
			})
			if terr != nil {
				return terr
			}
			tables = append(tables, table)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateTableIdentifier) {
			respondError(c, http.StatusConflict, CodeDuplicateTableIdentifier, "a table identifier already exists on this branch")
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.tables.create", "branch", strconv.FormatInt(branchID, 10), branch.OrganizationID, branchID, 0, gin.H{"count": len(tables)})
	c.JSON(http.StatusCreated, gin.H{"tables": platformTableResponses(tables)})
}
