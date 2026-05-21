package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	svc *services.SessionService
}

func NewSessionHandler(svc *services.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
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

	c.JSON(http.StatusCreated, gin.H{
		"session":     result.Session,
		"participant": result.Participant,
	})
}

func (h *SessionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
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

	participantID, err := participantIDFromHeader(c)
	if err != nil {
		respondValidationError(c, "X-Participant-ID header required")
		return
	}

	if err := h.svc.CloseSession(c.Request.Context(), id, participantID); err != nil {
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

	c.JSON(http.StatusCreated, participant)
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
func participantIDFromHeader(c *gin.Context) (int64, error) {
	raw := c.GetHeader("X-Participant-ID")
	if raw == "" {
		return 0, errors.New("missing")
	}
	return strconv.ParseInt(raw, 10, 64)
}
