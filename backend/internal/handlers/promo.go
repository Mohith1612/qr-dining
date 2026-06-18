package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PromoHandler struct {
	svc         *services.PromoService
	repos       *repository.Repos
	guestTokens *auth.GuestTokenService
	flags       config.FeatureFlags
	authz       *authz.Authorizer
}

func NewPromoHandler(svc *services.PromoService, repos *repository.Repos, guestTokens *auth.GuestTokenService, flags config.FeatureFlags, authorizer *authz.Authorizer) *PromoHandler {
	return &PromoHandler{svc: svc, repos: repos, guestTokens: guestTokens, flags: flags, authz: authorizer}
}

// POST /sessions/:id/promos/validate — public, rate-limited.
func (h *PromoHandler) ValidatePromo(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidationError(c, "invalid session id")
		return
	}
	if !requireGuestSession(c, h.guestTokens, h.repos, sessionID, h.flags.AuthGuestCredentialsRequired) {
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	sess, err := h.repos.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			respondError(c, http.StatusNotFound, CodeSessionNotFound, err.Error())
		} else {
			respondInternalError(c)
		}
		return
	}

	result, err := h.svc.ValidatePromo(c.Request.Context(), h.repos, services.ValidatePromoRequest{
		BranchID:   sess.BranchID,
		Code:       req.Code,
		OrderTotal: 0, // validate endpoint doesn't know cart total; min-order check skipped for preview
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPromoNotFound):
			respondError(c, http.StatusNotFound, CodePromoNotFound, "This promo code isn't valid right now.")
		case errors.Is(err, domain.ErrMinOrderNotMet):
			respondError(c, http.StatusUnprocessableEntity, CodeMinOrderNotMet, err.Error())
		case errors.Is(err, domain.ErrPromoExhausted):
			respondError(c, http.StatusConflict, CodePromoExhausted, "This offer has been claimed by too many guests.")
		case errors.Is(err, domain.ErrPromoAlreadyUsed):
			respondError(c, http.StatusConflict, CodePromoAlreadyUsed, "You've already used this offer.")
		default:
			respondInternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"promo_id":        result.PromoID,
		"discount_amount": result.DiscountAmount,
		"description":     result.Description,
	})
}

// GET /branches/:id/promos — staff protected.
func (h *PromoHandler) ListPromos(c *gin.Context) {
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
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, branchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, actor, authz.ActionBranchRead, authz.BranchResource(branchID, orgID)) {
		return
	}

	promos, err := h.svc.ListPromosForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, promos)
}

type createPromoRequest struct {
	Code            string  `json:"code" binding:"required"`
	Type            string  `json:"type" binding:"required"` // "flat_amount" | "percentage"
	Value           float64 `json:"value" binding:"required"`
	MinOrderAmount  float64 `json:"min_order_amount"`
	MaxUses         *int32  `json:"max_uses"`
	UsesPerPhone    int32   `json:"uses_per_phone"`
	ValidFrom       string  `json:"valid_from" binding:"required"`
	ValidUntil      string  `json:"valid_until" binding:"required"`
	TimeWindowStart *string `json:"time_window_start"` // "HH:MM" format
	TimeWindowEnd   *string `json:"time_window_end"`
	Description     *string `json:"description"`
}

// POST /branches/:id/promos — staff protected.
func (h *PromoHandler) CreatePromo(c *gin.Context) {
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
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, branchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, actor, authz.ActionPromoCreate, authz.PromoResource(0, branchID, orgID)) {
		return
	}

	var req createPromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	promoType := sqlc.PromoType(req.Type)
	if promoType != sqlc.PromoTypeFlatAmount && promoType != sqlc.PromoTypePercentage {
		respondValidationError(c, "type must be flat_amount or percentage")
		return
	}
	if req.UsesPerPhone == 0 {
		req.UsesPerPhone = 1
	}

	validFrom, err := time.Parse(time.RFC3339, req.ValidFrom)
	if err != nil {
		respondValidationError(c, "valid_from must be RFC3339")
		return
	}
	validUntil, err := time.Parse(time.RFC3339, req.ValidUntil)
	if err != nil {
		respondValidationError(c, "valid_until must be RFC3339")
		return
	}

	svcReq := services.CreatePromoRequest{
		BranchID:       branchID,
		Code:           strings.ToUpper(req.Code),
		Type:           promoType,
		Value:          req.Value,
		MinOrderAmount: req.MinOrderAmount,
		MaxUses:        req.MaxUses,
		UsesPerPhone:   req.UsesPerPhone,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		Description:    req.Description,
		CreatedBy:      &staffSession.StaffID,
	}

	if req.TimeWindowStart != nil && req.TimeWindowEnd != nil {
		start, errS := parseHHMM(*req.TimeWindowStart)
		end, errE := parseHHMM(*req.TimeWindowEnd)
		if errS != nil || errE != nil {
			respondValidationError(c, "time_window_start/end must be HH:MM")
			return
		}
		svcReq.TimeWindowStart = &start
		svcReq.TimeWindowEnd = &end
	}

	promo, err := h.svc.CreatePromo(c.Request.Context(), svcReq)
	if err != nil {
		if isDuplicateErr(err) {
			respondError(c, http.StatusConflict, CodeValidationError, "A promo with this code already exists.")
			return
		}
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusCreated, promo)
}

// DELETE /branches/:id/promos/:promo_id — staff protected (soft-delete).
func (h *PromoHandler) DeactivatePromo(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}
	promoID, err := strconv.ParseInt(c.Param("promo_id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid promo id")
		return
	}
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}
	promo, err := h.repos.GetPromoByID(c.Request.Context(), promoID)
	if err != nil {
		if errors.Is(err, domain.ErrPromoNotFound) {
			respondError(c, http.StatusNotFound, CodePromoNotFound, err.Error())
		} else {
			respondInternalError(c)
		}
		return
	}
	if promo.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	actor, ok := staffActorForRequest(c, h.repos, staffSession)
	if !ok {
		return
	}
	orgID, ok := restaurantIDForBranch(c, h.repos, promo.BranchID)
	if !ok {
		return
	}
	if !requireAuthorized(c, h.repos, h.authz, actor, authz.ActionPromoDeactivate, authz.PromoResource(promo.ID, promo.BranchID, orgID)) {
		return
	}

	if err := h.svc.DeactivatePromo(c.Request.Context(), promoID, branchID); err != nil {
		respondInternalError(c)
		return
	}
	c.Status(http.StatusNoContent)
}

// parseHHMM parses "HH:MM" into a duration since midnight.
func parseHHMM(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// isDuplicateErr reports whether err is a PostgreSQL unique constraint violation.
// Uses the same logic as repository.isDuplicateError but is package-local here.
func isDuplicateErr(err error) bool {
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate")
}
