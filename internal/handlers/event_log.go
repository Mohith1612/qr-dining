package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventLogHandler struct {
	repos *repository.Repos
}

func NewEventLogHandler(repos *repository.Repos) *EventLogHandler {
	return &EventLogHandler{repos: repos}
}

// GetSessionEvents returns the audit event timeline for a session (chronological, max 200).
func (h *EventLogHandler) GetSessionEvents(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	events, err := h.repos.GetEventsBySession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, events)
}

// GetBranchRecentEvents returns the 100 most recent events for a branch (newest first).
// Scoped to the authenticated staff member's branch.
func (h *EventLogHandler) GetBranchRecentEvents(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch id"})
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if ok && sess.BranchID != branchID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	events, err := h.repos.GetRecentEventsByBranch(c.Request.Context(), branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, events)
}
