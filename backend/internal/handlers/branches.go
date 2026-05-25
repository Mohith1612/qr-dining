package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
)

type BranchHandler struct {
	repos *repository.Repos
}

func NewBranchHandler(repos *repository.Repos) *BranchHandler {
	return &BranchHandler{repos: repos}
}

type updateBranchRequest struct {
	SessionTimeoutMinutes *int16 `json:"session_timeout_minutes"`
}

// GET /branches/:id — staff-protected.
func (h *BranchHandler) GetBranch(c *gin.Context) {
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

	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      branch.ID,
		"name":                    branch.Name,
		"session_timeout_minutes": branch.SessionTimeoutMinutes,
	})
}

// PATCH /branches/:id — owner only.
func (h *BranchHandler) UpdateBranch(c *gin.Context) {
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
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners and managers can update branch settings")
		return
	}

	var req updateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	if req.SessionTimeoutMinutes != nil {
		t := *req.SessionTimeoutMinutes
		if t < 15 || t > 480 {
			respondValidationError(c, "session_timeout_minutes must be between 15 and 480")
			return
		}
		if err := h.repos.UpdateBranchSessionTimeout(c.Request.Context(), branchID, t); err != nil {
			respondInternalError(c)
			return
		}
	}

	c.Status(http.StatusNoContent)
}
