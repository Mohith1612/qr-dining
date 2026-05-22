package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type SubscriptionHandler struct {
	repos   *repository.Repos
	subSvc  *services.SubscriptionService
}

func NewSubscriptionHandler(repos *repository.Repos, subSvc *services.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{repos: repos, subSvc: subSvc}
}

// ListPlans returns all subscription plans.
// GET /plans — public.
func (h *SubscriptionHandler) ListPlans(c *gin.Context) {
	plans, err := h.repos.ListPlans(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

// GetSubscription returns the restaurant's current subscription and plan features.
// GET /restaurants/:id/subscription — staff-protected.
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	restaurantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid restaurant id")
		return
	}

	// Verify the requesting staff belongs to a branch in this restaurant.
	staffSession, ok := middleware.GetStaffSession(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "staff authentication required")
		return
	}

	branch, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), staffSession.BranchID)
	if err != nil {
		if errors.Is(err, domain.ErrTenantNotFound) {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		respondInternalError(c)
		return
	}
	if branch.ID != restaurantID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	sub, err := h.repos.GetSubscriptionByRestaurant(c.Request.Context(), restaurantID)
	if err != nil {
		if errors.Is(err, domain.ErrPlanNotFound) {
			respondError(c, http.StatusNotFound, CodePlanNotFound, "no subscription found")
			return
		}
		respondInternalError(c)
		return
	}

	features := repository.DecodePlanFeatures(sub.PlanFeaturesJson)

	c.JSON(http.StatusOK, gin.H{
		"subscription": sub,
		"features":     features,
	})
}
