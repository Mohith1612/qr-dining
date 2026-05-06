package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AssistanceHandler struct {
	svc *services.AssistanceService
}

func NewAssistanceHandler(svc *services.AssistanceService) *AssistanceHandler {
	return &AssistanceHandler{svc: svc}
}

type requestAssistanceRequest struct {
	TableID       int64  `json:"table_id" binding:"required"`
	ParticipantID int64  `json:"participant_id"`
	Type          string `json:"type"` // waiter | bill | other
}

func (h *AssistanceHandler) Request(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
		return
	}

	var req requestAssistanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reqType := sqlc.AssistanceTypeWaiter
	switch req.Type {
	case "bill":
		reqType = sqlc.AssistanceTypeBill
	case "other":
		reqType = sqlc.AssistanceTypeOther
	}

	ar, err := h.svc.Request(c.Request.Context(), sessionID, req.TableID, req.ParticipantID, reqType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, ar)
}

func (h *AssistanceHandler) Acknowledge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ar, err := h.svc.Acknowledge(c.Request.Context(), id)
	if err != nil {
		assistanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ar)
}

func (h *AssistanceHandler) Resolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ar, err := h.svc.Resolve(c.Request.Context(), id)
	if err != nil {
		assistanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ar)
}

func assistanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrAssistanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidAssistanceTransition):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
