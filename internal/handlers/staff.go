package handlers

import (
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"errors"
)

type StaffHandler struct {
	svc *services.StaffService
}

func NewStaffHandler(svc *services.StaffService) *StaffHandler {
	return &StaffHandler{svc: svc}
}

type staffAuthRequest struct {
	BranchID int64  `json:"branch_id" binding:"required"`
	PIN      string `json:"pin" binding:"required,min=4,max=6"`
}

func (h *StaffHandler) Authenticate(c *gin.Context) {
	var req staffAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.svc.Authenticate(c.Request.Context(), req.BranchID, req.PIN)
	if err != nil {
		if errors.Is(err, domain.ErrParticipantUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, session)
}
