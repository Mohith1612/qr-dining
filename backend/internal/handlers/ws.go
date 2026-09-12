package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WSHandler struct {
	hub         *ws.Hub
	repos       *repository.Repos
	metrics     *observability.Metrics
	guestTokens *auth.GuestTokenService
	tickets     *redisPkg.WSTicketStore
	flags       config.FeatureFlags
}

func NewWSHandler(hub *ws.Hub, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, tickets *redisPkg.WSTicketStore, flags config.FeatureFlags) *WSHandler {
	return &WSHandler{hub: hub, repos: repos, metrics: metrics, guestTokens: guestTokens, tickets: tickets, flags: flags}
}

// Upgrade upgrades the HTTP connection to WebSocket.
// Required query params: session_id (UUID), participant_id (int64).
func (h *WSHandler) Upgrade(c *gin.Context) {
	if ticket := c.Query("ticket"); ticket != "" {
		h.upgradeWithTicket(c, ticket)
		return
	}
	if h.flags.WSTicketAuthRequired {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "websocket ticket required")
		return
	}

	sessionIDStr := c.Query("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		respondValidationError(c, "invalid session_id")
		return
	}

	participantIDStr := c.Query("participant_id")
	participantID, err := strconv.ParseInt(participantIDStr, 10, 64)
	if (err != nil || participantID <= 0) && guestTokenFromHeader(c) == "" {
		respondValidationError(c, "invalid participant_id")
		return
	}
	if participantID > 0 {
		recordLegacyIdentityUsage(h.metrics, legacyMechanismWSQueryParticipantID, legacyEndpointWebSocket)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, participantID, h.flags.AuthGuestCredentialsRequired || h.flags.WSTicketAuthRequired)
	if !ok {
		return
	}

	// Validate session is active and participant belongs to it.
	sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeSessionNotFound, "session not found")
		return
	}
	if !sessionServesRealtime(sess.Status) {
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

func recordWSTicketConsumeFailure(m *observability.Metrics, reason string) {
	if m != nil && m.WSTicketConsumeFailedTotal != nil {
		m.WSTicketConsumeFailedTotal.WithLabelValues(reason).Inc()
	}
}

// sessionServesRealtime reports whether a session in this status may hold a live
// WebSocket. Both statuses are non-terminal and both have events the guest needs:
// payment_pending is exactly when PAYMENT_COMPLETED / PAYMENT_CANCELLED and the
// remaining ORDER_* transitions arrive, so refusing the socket there blinds the
// guest during settlement — the one moment they are watching hardest (F-07).
//
// The payment freeze is a CART freeze (domain.IsSessionCartFrozen), enforced at
// the mutation endpoints; it was never meant to be a connectivity freeze.
// awaiting_reactivation stays out: the client has a dedicated recovery path for
// it (snapshot reactivates, then reconnects), and a refused ticket is that
// path's trigger.
func sessionServesRealtime(status sqlc.SessionStatus) bool {
	return status == sqlc.SessionStatusActive || status == sqlc.SessionStatusPaymentPending
}

func (h *WSHandler) upgradeWithTicket(c *gin.Context, ticket string) {
	claims, err := h.tickets.Consume(c.Request.Context(), ticket)
	if err != nil {
		if errors.Is(err, redisPkg.ErrWSTicketInvalid) {
			recordWSTicketConsumeFailure(h.metrics, "invalid")
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid websocket ticket")
			return
		}
		recordWSTicketConsumeFailure(h.metrics, "internal")
		respondInternalError(c)
		return
	}

	sess, err := h.repos.GetSessionByID(c.Request.Context(), claims.SessionID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeSessionNotFound, "session not found")
		return
	}
	if !sessionServesRealtime(sess.Status) || sess.BranchID != claims.BranchID {
		respondError(c, http.StatusConflict, CodeSessionClosed, "session is not active")
		return
	}
	organization, err := h.repos.GetOrganizationByBranchID(c.Request.Context(), sess.BranchID)
	if err != nil || organization.ID != claims.OrganizationID {
		respondError(c, http.StatusForbidden, CodeForbidden, "ticket scope does not match session")
		return
	}

	participant, err := h.repos.GetParticipantByID(c.Request.Context(), claims.ParticipantID)
	if err != nil || participant.SessionID != claims.SessionID || participant.CredentialVersion != claims.CredentialVersion {
		respondError(c, http.StatusForbidden, CodeParticipantNotFound, "participant not in session")
		return
	}
	if participant.RevokedAt.Valid {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential has been revoked")
		return
	}

	if err := h.hub.Upgrade(c.Writer, c.Request, claims.SessionID, claims.ParticipantID); err != nil {
		return
	}
}
