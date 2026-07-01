package handlers

import (
	"net/http"
	"strconv"

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
