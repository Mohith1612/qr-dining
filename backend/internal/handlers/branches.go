package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
)

type BranchHandler struct {
	repos *repository.Repos
}

func NewBranchHandler(repos *repository.Repos) *BranchHandler {
	return &BranchHandler{repos: repos}
}

type updateBranchRequest struct {
	SessionTimeoutMinutes *int16   `json:"session_timeout_minutes"`
	LogoURL               *string  `json:"logo_url"`
	OrderPrefix           *string  `json:"order_prefix"`
	Theme                 *string  `json:"theme"`
	TaxRate               *float64 `json:"tax_rate"`
	ServiceChargeRate     *float64 `json:"service_charge_rate"`
	IncludeTaxInPrice     *bool    `json:"include_tax_in_price"`
}

var validThemes = map[string]bool{
	"dark-luxury":    true,
	"modern-minimal": true,
}

// GET /branches/:id — staff-protected.
func (h *BranchHandler) GetBranch(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}

	branch, err := h.repos.GetBranchByID(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}

	theme := "dark-luxury"
	var taxRate, serviceChargeRate float64
	var includeTaxInPrice bool
	if restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); err == nil {
		var settings map[string]any
		if json.Unmarshal(restaurant.SettingsJson, &settings) == nil {
			if t, ok := settings["theme"].(string); ok && validThemes[t] {
				theme = t
			}
			if v, ok := settings["tax_rate"].(float64); ok {
				taxRate = v
			}
			if v, ok := settings["service_charge_rate"].(float64); ok {
				serviceChargeRate = v
			}
			if v, ok := settings["include_tax_in_price"].(bool); ok {
				includeTaxInPrice = v
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      branch.ID,
		"organization_id":         branch.OrganizationID,
		"name":                    branch.Name,
		"branch_code":             branch.BranchCode,
		"status":                  branch.Status,
		"support_metadata":        branch.SupportMetadataJson,
		"session_timeout_minutes": branch.SessionTimeoutMinutes,
		"order_prefix":            branch.OrderPrefix,
		"theme":                   theme,
		"tax_rate":                taxRate,
		"service_charge_rate":     serviceChargeRate,
		"include_tax_in_price":    includeTaxInPrice,
	})
}

// PATCH /branches/:id — owner only.
func (h *BranchHandler) UpdateBranch(c *gin.Context) {
	branchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondValidationError(c, "invalid branch id")
		return
	}

	sess, ok := middleware.GetStaffSession(c)
	if !ok || sess.BranchID != branchID {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	if sess.Role != sqlc.StaffRoleOwner && sess.Role != sqlc.StaffRoleManager {
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners and managers can update branch settings")
		return
	}

	var req updateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err.Error())
		return
	}

	if req.SessionTimeoutMinutes != nil {
		t := *req.SessionTimeoutMinutes
		if t < 15 || t > 480 {
			respondValidationError(c, "session_timeout_minutes must be between 15 and 480")
			return
		}
		if err := h.repos.UpdateBranchSessionTimeout(c.Request.Context(), branchID, t); err != nil {
			respondInternalError(c)
			return
		}
	}

	if req.LogoURL != nil {
		if err := h.repos.UpdateRestaurantLogoByBranchID(c.Request.Context(), branchID, *req.LogoURL); err != nil {
			respondInternalError(c)
			return
		}
	}

	if req.OrderPrefix != nil {
		p := strings.ToUpper(strings.TrimSpace(*req.OrderPrefix))
		matched, _ := regexp.MatchString(`^[A-Z]{2,4}$`, p)
		if !matched {
			respondValidationError(c, "order_prefix must be 2–4 uppercase letters")
			return
		}
		if err := h.repos.UpdateBranchOrderPrefix(c.Request.Context(), branchID, p); err != nil {
			respondInternalError(c)
			return
		}
	}

	if req.Theme != nil {
		if !validThemes[*req.Theme] {
			respondValidationError(c, "theme must be one of: dark-luxury, modern-minimal")
			return
		}
		if err := h.repos.UpdateRestaurantThemeByBranchID(c.Request.Context(), branchID, *req.Theme); err != nil {
			respondInternalError(c)
			return
		}
	}

	if req.TaxRate != nil || req.ServiceChargeRate != nil || req.IncludeTaxInPrice != nil {
		// Read current values first so we only update what was sent.
		taxRate, serviceChargeRate, includeTaxInPrice := 0.0, 0.0, false
		if restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); err == nil {
			var settings map[string]any
			if json.Unmarshal(restaurant.SettingsJson, &settings) == nil {
				if v, ok := settings["tax_rate"].(float64); ok {
					taxRate = v
				}
				if v, ok := settings["service_charge_rate"].(float64); ok {
					serviceChargeRate = v
				}
				if v, ok := settings["include_tax_in_price"].(bool); ok {
					includeTaxInPrice = v
				}
			}
		}
		if req.TaxRate != nil {
			if *req.TaxRate < 0 || *req.TaxRate > 0.5 {
				respondValidationError(c, "tax_rate must be between 0.0 and 0.5")
				return
			}
			taxRate = *req.TaxRate
		}
		if req.ServiceChargeRate != nil {
			if *req.ServiceChargeRate < 0 || *req.ServiceChargeRate > 0.5 {
				respondValidationError(c, "service_charge_rate must be between 0.0 and 0.5")
				return
			}
			serviceChargeRate = *req.ServiceChargeRate
		}
		if req.IncludeTaxInPrice != nil {
			includeTaxInPrice = *req.IncludeTaxInPrice
		}
		if err := h.repos.UpdateRestaurantBillingByBranchID(c.Request.Context(), branchID, taxRate, serviceChargeRate, includeTaxInPrice); err != nil {
			respondInternalError(c)
			return
		}
	}

	c.Status(http.StatusNoContent)
}
