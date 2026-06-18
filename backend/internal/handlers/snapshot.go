package handlers

import (
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
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

	snapshot, err := h.svc.GetSnapshot(c.Request.Context(), sessionID)
	if err != nil {
		sessionError(c, err)
		return
	}

	c.JSON(http.StatusOK, snapshot)
}
