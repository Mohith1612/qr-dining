package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CustomerHandler struct {
	svc         *services.CustomerService
	repos       *repository.Repos
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
}

func NewCustomerHandler(svc *services.CustomerService, repos *repository.Repos, guestTokens *auth.GuestTokenService, flags config.FeatureFlags) *CustomerHandler {
	return &CustomerHandler{svc: svc, repos: repos, guestTokens: guestTokens, flags: flags}
}

type linkCustomerRequest struct {
	Phone       string `json:"phone" binding:"required"`
	DisplayName string `json:"display_name"`
}

// POST /sessions/:id/customer — guest opt-in after payment
func (h *CustomerHandler) LinkCustomer(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, sessionID, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	var req linkCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	err = h.svc.LinkCustomer(c.Request.Context(), services.LinkCustomerRequest{
		SessionID:   sessionID,
		Phone:       req.Phone,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidPhone):
			respondError(c, http.StatusBadRequest, CodeInvalidPhone, err.Error())
		case errors.Is(err, domain.ErrFeatureDisabled):
			respondError(c, http.StatusNotFound, CodeFeatureDisabled, err.Error())
		case errors.Is(err, domain.ErrSessionNotFound):
			respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /branches/:id/customers?q=<partial_phone> — staff only
func (h *CustomerHandler) SearchCustomers(c *gin.Context) {
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	if staffSession.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	prefix := c.Query("q")
	customers, err := h.svc.SearchCustomers(c.Request.Context(), restaurant.ID, prefix)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"customers": customers})
}

// GET /customers/:id/history — staff only
func (h *CustomerHandler) GetCustomerHistory(c *gin.Context) {
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	customerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid customer id")
		return
	}

	restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), staffSession.BranchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	history, err := h.svc.GetCustomerHistory(c.Request.Context(), customerID, restaurant.ID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history})
}

// DELETE /customers/:id — staff only (owner/manager)
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	if staffSession.Role != "owner" && staffSession.Role != "manager" {
		respondError(c, http.StatusForbidden, CodeForbidden, "owner or manager required")
		return
	}

	customerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid customer id")
		return
	}

	restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), staffSession.BranchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	if err := h.svc.DeleteCustomer(c.Request.Context(), customerID, restaurant.ID); err != nil {
		respondInternalError(c)
		return
	}

	c.Status(http.StatusNoContent)
}
