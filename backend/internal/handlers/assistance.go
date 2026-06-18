package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AssistanceHandler struct {
	svc     *services.AssistanceService
	metrics *observability.Metrics
}

func NewAssistanceHandler(svc *services.AssistanceService, metrics *observability.Metrics) *AssistanceHandler {
	return &AssistanceHandler{svc: svc, metrics: metrics}
}

type requestAssistanceRequest struct {
	TableID       int64  `json:"table_id" binding:"required"`
	ParticipantID int64  `json:"participant_id"`
	Type          string `json:"type"` // waiter | bill | other
}

func (h *AssistanceHandler) Request(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}

	var req requestAssistanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	recordLegacyIdentityUsage(h.metrics, legacyMechanismBodyParticipantID, legacyEndpointAssistance)

	reqType := sqlc.AssistanceTypeWaiter
	switch req.Type {
	case "bill":
		reqType = sqlc.AssistanceTypeBill
	case "other":
		reqType = sqlc.AssistanceTypeOther
	}

	ar, err := h.svc.Request(c.Request.Context(), sessionID, req.TableID, req.ParticipantID, reqType)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusCreated, ar)
}

func (h *AssistanceHandler) Acknowledge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid id")
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	ar, err := h.svc.Acknowledge(c.Request.Context(), id, staffSession.StaffID)
	if err != nil {
		assistanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ar)
}

func (h *AssistanceHandler) Resolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid id")
		return
	}

	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	ar, err := h.svc.Resolve(c.Request.Context(), id, staffSession.StaffID)
	if err != nil {
		assistanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ar)
}

func (h *AssistanceHandler) ListActiveForBranch(c *gin.Context) {
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

	requests, err := h.svc.ListActiveForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, requests)
}

func assistanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrAssistanceNotFound):
		respondError(c, http.StatusNotFound, CodeAssistanceNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidAssistanceTransition):
		respondError(c, http.StatusUnprocessableEntity, CodeInvalidAssistTransition, err.Error())
	default:
		respondInternalError(c)
	}
}
