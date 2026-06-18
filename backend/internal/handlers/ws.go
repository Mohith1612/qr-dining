package handlers

import (
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WSHandler struct {
	hub     *ws.Hub
	repos   *repository.Repos
	metrics *observability.Metrics
}

func NewWSHandler(hub *ws.Hub, repos *repository.Repos, metrics *observability.Metrics) *WSHandler {
	return &WSHandler{hub: hub, repos: repos, metrics: metrics}
}

// Upgrade upgrades the HTTP connection to WebSocket.
// Required query params: session_id (UUID), participant_id (int64).
func (h *WSHandler) Upgrade(c *gin.Context) {
	sessionIDStr := c.Query("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		respondValidationError(c, "invalid session_id")
		return
	}

	participantIDStr := c.Query("participant_id")
	participantID, err := strconv.ParseInt(participantIDStr, 10, 64)
	if err != nil || participantID <= 0 {
		respondValidationError(c, "invalid participant_id")
		return
	}
	recordLegacyIdentityUsage(h.metrics, legacyMechanismWSQueryParticipantID, legacyEndpointWebSocket)

	// Validate session is active and participant belongs to it.
	sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeSessionNotFound, "session not found")
		return
	}
	if sess.Status != "active" {
		respondError(c, http.StatusConflict, CodeSessionClosed, "session is not active")
		return
	}

	participant, err := h.repos.GetParticipantByID(c.Request.Context(), participantID)
	if err != nil || participant.SessionID != sessionID {
		respondError(c, http.StatusForbidden, CodeParticipantNotFound, "participant not in session")
		return
	}

	if err := h.hub.Upgrade(c.Writer, c.Request, sessionID, participantID); err != nil {
		// Upgrade itself sends the error response.
		return
	}
}
