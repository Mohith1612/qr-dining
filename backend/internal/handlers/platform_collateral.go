package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

// Premium QR collateral (platform operator). Read-only roles may inspect the branch's
// tables and saved collateral config; super_admin may set it. All reads/writes are
// audited via the platform audit log, consistent with the theme/branding endpoints.

// ListBranchTables returns a branch's tables (incl. QR tokens) for collateral generation.
// GET /platform/branches/:branch_id/tables
func (h *PlatformHandler) ListBranchTables(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
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
	tables, err := h.repos.ListTablesForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.tables.list", "branch", strconv.FormatInt(branch.ID, 10), branch.OrganizationID, branch.ID, 0, gin.H{"count": len(tables)})
	c.JSON(http.StatusOK, gin.H{"tables": platformTableResponses(tables)})
}

// GetBranchCollateral returns the saved collateral config (or defaults) for a branch.
// GET /platform/branches/:branch_id/collateral
func (h *PlatformHandler) GetBranchCollateral(c *gin.Context) {
	session, ok := h.requireAnyPlatformRole(c, services.PlatformRoleSupportAdmin, services.PlatformRoleBillingAdmin, services.PlatformRoleReadOnlyAuditor)
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
	cfg, err := h.collateral.GetForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.collateral.read", "branch", strconv.FormatInt(branch.ID, 10), branch.OrganizationID, branch.ID, 0, gin.H{})
	c.JSON(http.StatusOK, gin.H{"collateral": cfg, "formats": services.AllowedCollateralFormats()})
}

// SetBranchCollateral validates and persists the collateral config for a branch.
// PUT /platform/branches/:branch_id/collateral — super_admin.
func (h *PlatformHandler) SetBranchCollateral(c *gin.Context) {
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
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	uid := session.PlatformUserID
	cfg, err := h.collateral.SetForBranch(c.Request.Context(), branchID, raw, &uid)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCollateralConfig) {
			respondValidationError(c, err.Error())
			return
		}
		respondInternalError(c)
		return
	}
	h.logPlatformAudit(c, session.PlatformUserID, "platform.branches.collateral.set", "branch", strconv.FormatInt(branch.ID, 10), branch.OrganizationID, branch.ID, 0, gin.H{"format": cfg.Format})
	c.JSON(http.StatusOK, gin.H{"collateral": cfg})
}
