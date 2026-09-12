package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type MenuHandler struct {
	svc *services.MenuService
}

func NewMenuHandler(svc *services.MenuService) *MenuHandler {
	return &MenuHandler{svc: svc}
}

func (h *MenuHandler) GetMenu(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	menu, err := h.svc.GetFullMenu(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, menu)
}

func (h *MenuHandler) GetTableByQR(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		respondValidationError(c, "missing token")
		return
	}

	resp, err := h.svc.GetTableWithActiveSession(c.Request.Context(), token)
	if err != nil {
		// A suspended tenant must not read as "table not found": the guest is at
		// a real table and needs to know the restaurant isn't serving, not that
		// their QR code is broken.
		switch {
		case errors.Is(err, domain.ErrOrganizationSuspended):
			respondError(c, http.StatusForbidden, CodeOrganizationSuspended, err.Error())
		case errors.Is(err, domain.ErrBranchSuspended):
			respondError(c, http.StatusForbidden, CodeBranchSuspended, err.Error())
		default:
			respondError(c, http.StatusNotFound, CodeSessionNotFound, "table not found")
		}
		return
	}
	c.JSON(http.StatusOK, resp)
}
