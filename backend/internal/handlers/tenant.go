package handlers

import (
	"errors"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type TenantHandler struct {
	repos *repository.Repos
	theme *services.ThemeService
}

func NewTenantHandler(repos *repository.Repos, theme *services.ThemeService) *TenantHandler {
	return &TenantHandler{repos: repos, theme: theme}
}

// GetBySlug resolves a restaurant by its URL slug.
// GET /tenants/by-slug/:slug
// Public — used by the frontend to initialize tenant context on boot.
func (h *TenantHandler) GetBySlug(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		respondValidationError(c, "slug is required")
		return
	}

	restaurant, err := h.repos.GetRestaurantBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, domain.ErrTenantNotFound) {
			respondError(c, http.StatusNotFound, "TENANT_NOT_FOUND", "restaurant not found")
			return
		}
		respondInternalError(c)
		return
	}

	// Resolve the structured theme (preset + custom tokens). Bridges legacy
	// settings_json.theme when no tenant_themes row exists, so this is always
	// populated. settings is left intact for backward compatibility.
	theme, err := h.theme.GetThemeForRestaurant(c.Request.Context(), restaurant.ID)
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              restaurant.ID,
		"organization_id": restaurant.OrganizationID,
		"name":            restaurant.Name,
		"slug":            restaurant.Slug,
		"settings":        restaurant.SettingsJson,
		"theme":           theme,
	})
}
