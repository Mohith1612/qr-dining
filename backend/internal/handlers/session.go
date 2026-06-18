package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	svc         *services.SessionService
	repos       *repository.Repos
	metrics     *observability.Metrics
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
}

func NewSessionHandler(svc *services.SessionService, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, flags config.FeatureFlags) *SessionHandler {
	return &SessionHandler{svc: svc, repos: repos, metrics: metrics, guestTokens: guestTokens, flags: flags}
}

type createSessionRequest struct {
	TableID           int64  `json:"table_id" binding:"required"`
	DisplayName       string `json:"display_name" binding:"required,min=1,max=50"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	result, err := h.svc.CreateSession(c.Request.Context(), req.TableID, req.DisplayName, req.DeviceFingerprint)
	if err != nil {
		sessionError(c, err)
		return
	}

	token, err := h.issueGuestToken(c, result.Session, result.Participant)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session":            result.Session,
		"participant":        result.Participant,
		"guest_access_token": token,
	})
}

func (h *SessionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, id, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	sess, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}

	c.JSON(http.StatusOK, sess)
}

type closeSessionRequest struct {
	ParticipantID int64 `json:"participant_id" binding:"required"`
}

func (h *SessionHandler) Close(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	participantID, hasLegacyParticipant := participantIDFromHeader(c)
	if !hasLegacyParticipant && guestTokenFromHeader(c) == "" {
		respondValidationError(c, "X-Participant-ID header required")
		return
	}
	if hasLegacyParticipant {
		recordLegacyIdentityUsage(h.metrics, legacyMechanismHeaderParticipantID, legacyEndpointSession)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, id, participantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}

	if err := h.svc.CloseSession(c.Request.Context(), id, &participantID); err != nil {
		sessionError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type joinSessionRequest struct {
	DisplayName       string `json:"display_name" binding:"required,min=1,max=50"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

func (h *SessionHandler) Join(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req joinSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	participant, err := h.svc.JoinSession(c.Request.Context(), id, req.DisplayName, req.DeviceFingerprint)
	if err != nil {
		sessionError(c, err)
		return
	}

	sess, err := h.svc.GetSession(c.Request.Context(), id)
	if err != nil {
		sessionError(c, err)
		return
	}
	token, err := h.issueGuestToken(c, sess, participant)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session":            sess,
		"participant":        participant,
		"guest_access_token": token,
	})
}

func (h *SessionHandler) ListActiveForBranch(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	sessions, err := h.svc.ListActiveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func sessionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrSessionNotFound):
		respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
	case errors.Is(err, domain.ErrSessionClosed):
		respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
	case errors.Is(err, domain.ErrSessionAlreadyActive):
		respondError(c, http.StatusConflict, CodeSessionAlreadyActive, err.Error())
	case errors.Is(err, domain.ErrNotSessionHost):
		respondError(c, http.StatusForbidden, CodeNotSessionHost, err.Error())
	case errors.Is(err, domain.ErrTableNotFound):
		respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
	default:
		respondInternalError(c)
	}
}

// participantIDFromHeader reads X-Participant-ID from the request header.
func participantIDFromHeader(c *gin.Context) (int64, bool) {
	raw := c.GetHeader("X-Participant-ID")
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil
}

func (h *SessionHandler) issueGuestToken(c *gin.Context, sess sqlc.Session, participant sqlc.SessionParticipant) (string, error) {
	organization, err := h.repos.GetOrganizationByBranchID(c.Request.Context(), sess.BranchID)
	if err != nil {
		return "", err
	}
	role := "guest"
	if participant.IsHost {
		role = "host"
	}
	return h.guestTokens.Issue(auth.GuestClaims{
		SessionID:         sess.ID,
		BranchID:          sess.BranchID,
		TableID:           sess.TableID,
		OrganizationID:    organization.ID,
		Role:              role,
		ParticipantID:     participant.ID,
		CredentialVersion: participant.CredentialVersion,
	})
}
