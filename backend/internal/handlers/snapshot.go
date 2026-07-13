package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SnapshotHandler struct {
	svc         *services.SessionService
	repos       *repository.Repos
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
}

func NewSnapshotHandler(svc *services.SessionService, repos *repository.Repos, guestTokens *auth.GuestTokenService, flags config.FeatureFlags) *SnapshotHandler {
	return &SnapshotHandler{svc: svc, repos: repos, guestTokens: guestTokens, flags: flags}
}

// GetSnapshot returns the full authoritative current state of a session.
// Clients call this endpoint on WebSocket reconnect to reconcile local state
// against the PostgreSQL source of truth.
func (h *SnapshotHandler) GetSnapshot(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, sessionID, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	var lastSequence int64
	if raw := c.Query("last_sequence"); raw != "" {
		lastSequence, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || lastSequence < 0 {
			respondValidationError(c, "invalid last_sequence")
			return
		}
	}

	snapshot, err := h.svc.GetSnapshot(c.Request.Context(), sessionID, lastSequence)
	if err != nil {
		if errors.Is(err, domain.ErrSessionTerminalReadExpired) {
			respondError(c, http.StatusGone, CodeSessionEnded, err.Error())
			return
		}
		sessionError(c, err)
		return
	}

	// Never serialize the guest credential (session_token) in the snapshot. With
	// AUTH_GUEST_CREDENTIALS_REQUIRED=false this endpoint is reachable without a
	// token, so stripping it here is the permanent, flag-independent fix (F-8).
	snapshot.Session = guestSafeSession(snapshot.Session)

	c.JSON(http.StatusOK, snapshot)
}
