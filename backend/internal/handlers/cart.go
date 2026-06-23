package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CartHandler struct {
	svc         *services.CartService
	repos       *repository.Repos
	metrics     *observability.Metrics
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
}

func NewCartHandler(svc *services.CartService, repos *repository.Repos, metrics *observability.Metrics, guestTokens *auth.GuestTokenService, flags config.FeatureFlags) *CartHandler {
	return &CartHandler{svc: svc, repos: repos, metrics: metrics, guestTokens: guestTokens, flags: flags}
}

func (h *CartHandler) GetCart(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
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
		recordLegacyIdentityUsage(h.metrics, legacyMechanismHeaderParticipantID, legacyEndpointCart)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, participantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}

	result, err := h.svc.GetCart(c.Request.Context(), sessionID, participantID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

type addCartItemRequest struct {
	MenuItemID  int64   `json:"menu_item_id" binding:"required"`
	Quantity    int16   `json:"quantity" binding:"required,min=1,max=99"`
	ModifierIDs []int64 `json:"modifier_ids"`
	Note        string  `json:"note"`
}

func (h *CartHandler) AddItem(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
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
		recordLegacyIdentityUsage(h.metrics, legacyMechanismHeaderParticipantID, legacyEndpointCart)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, participantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}

	var req addCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	item, err := h.svc.AddItem(c.Request.Context(), services.AddItemRequest{
		SessionID:     sessionID,
		ParticipantID: participantID,
		MenuItemID:    req.MenuItemID,
		Quantity:      req.Quantity,
		ModifierIDs:   req.ModifierIDs,
		Note:          req.Note,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrMenuItemNotFound):
			respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
		case errors.Is(err, domain.ErrModifierNotFound):
			respondError(c, http.StatusNotFound, CodeMenuItemNotFound, err.Error())
		case errors.Is(err, domain.ErrMenuItemUnavailable):
			respondError(c, http.StatusUnprocessableEntity, CodeMenuItemUnavailable, err.Error())
		case errors.Is(err, domain.ErrPaymentInProgress):
			respondError(c, http.StatusConflict, CodePaymentInProgress, err.Error())
		case errors.Is(err, domain.ErrSessionClosed):
			respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CartHandler) RemoveItem(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid item_id")
		return
	}
	participantID, hasLegacyParticipant := participantIDFromHeader(c)
	if !hasLegacyParticipant && guestTokenFromHeader(c) == "" {
		respondValidationError(c, "X-Participant-ID header required")
		return
	}
	if hasLegacyParticipant {
		recordLegacyIdentityUsage(h.metrics, legacyMechanismHeaderParticipantID, legacyEndpointCart)
	}
	participantID, ok := guestParticipantID(c, h.guestTokens, h.repos, sessionID, participantID, h.flags.AuthGuestCredentialsRequired)
	if !ok {
		return
	}

	if err := h.svc.RemoveItem(c.Request.Context(), sessionID, participantID, itemID); err != nil {
		switch {
		case errors.Is(err, domain.ErrCartItemNotFound):
			respondError(c, http.StatusNotFound, CodeCartItemNotFound, err.Error())
		case errors.Is(err, domain.ErrPaymentInProgress):
			respondError(c, http.StatusConflict, CodePaymentInProgress, err.Error())
		case errors.Is(err, domain.ErrSessionClosed):
			respondError(c, http.StatusConflict, CodeSessionClosed, err.Error())
		default:
			respondInternalError(c)
		}
		return
	}
	c.Status(http.StatusNoContent)
}
