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
// Scoped to the authenticated staff member's branch: the session must belong to it.
func (h *EventLogHandler) GetSessionEvents(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	session, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeSessionNotFound, "session not found")
		return
	}
	if session.BranchID != sess.BranchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	events, err := h.repos.GetEventsBySession(c.Request.Context(), sessionID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, events)
}

// GetBranchRecentEvents returns the 100 most recent events for a branch (newest first).
// Scoped to the authenticated staff member's branch.
func (h *EventLogHandler) GetBranchRecentEvents(c *gin.Context) {
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

	events, err := h.repos.GetRecentEventsByBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, events)
}
