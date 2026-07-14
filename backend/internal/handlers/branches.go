package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
)

type BranchHandler struct {
	repos      *repository.Repos
	theme      *services.ThemeService
	collateral *services.CollateralService
	audit      *audit.Writer
}

func NewBranchHandler(repos *repository.Repos, themeSvc *services.ThemeService, collateralSvc *services.CollateralService, auditWriter *audit.Writer) *BranchHandler {
	return &BranchHandler{repos: repos, theme: themeSvc, collateral: collateralSvc, audit: auditWriter}
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
	var restaurantName, logoURL string
	if restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID); err == nil {
		restaurantName = restaurant.Name
		if restaurant.LogoUrl.Valid {
			logoURL = restaurant.LogoUrl.String
		}
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
		"restaurant_name":         restaurantName,
		"logo_url":                logoURL,
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
		// Unified write-path: the structured tenant_themes row is the authoritative
		// source (preset only → no custom tokens → no entitlement gate). We write it
		// first so the read path (which prefers tenant_themes) always reflects the
		// operator's choice; the legacy settings_json.theme write below is preserved
		// for backward compatibility.
		restaurant, err := h.repos.GetRestaurantByBranchID(c.Request.Context(), branchID)
		if err != nil {
			respondInternalError(c)
			return
		}
		if _, err := h.theme.SetTheme(c.Request.Context(), restaurant.OrganizationID, *req.Theme, nil, nil); err != nil {
			respondInternalError(c)
			return
		}
		if err := h.repos.UpdateRestaurantThemeByBranchID(c.Request.Context(), branchID, *req.Theme); err != nil {
			respondInternalError(c)
			return
		}
	}

	updatedFields := []string{}
	if req.SessionTimeoutMinutes != nil {
		updatedFields = append(updatedFields, "session_timeout_minutes")
	}
	if req.LogoURL != nil {
		updatedFields = append(updatedFields, "logo_url")
	}
	if req.OrderPrefix != nil {
		updatedFields = append(updatedFields, "order_prefix")
	}
	if req.Theme != nil {
		updatedFields = append(updatedFields, "theme")
	}
	if req.TaxRate != nil {
		updatedFields = append(updatedFields, "tax_rate")
	}
	if req.ServiceChargeRate != nil {
		updatedFields = append(updatedFields, "service_charge_rate")
	}
	if req.IncludeTaxInPrice != nil {
		updatedFields = append(updatedFields, "include_tax_in_price")
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

	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     branchID,
		ResourceType: audit.ResourceBranch,
		ResourceID:   audit.IDStr(branchID),
		Action:       audit.ActionBranchSettingsUpdate,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskMedium,
		Metadata:     map[string]any{"fields_updated": updatedFields},
	})
	c.Status(http.StatusNoContent)
}

// GET /branches/:id/collateral — staff-protected. Returns the saved QR collateral
// config (or defaults) for the branch.
func (h *BranchHandler) GetCollateral(c *gin.Context) {
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
	cfg, err := h.collateral.GetForBranch(c.Request.Context(), branchID)
	if err != nil {
		respondInternalError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"collateral": cfg, "formats": services.AllowedCollateralFormats()})
}

// PUT /branches/:id/collateral — owner/manager only. Validates and persists the QR
// collateral config. Writes the same branch_collateral row the platform operator does.
func (h *BranchHandler) UpdateCollateral(c *gin.Context) {
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
		respondError(c, http.StatusForbidden, CodeForbidden, "only owners and managers can update collateral")
		return
	}
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondValidationError(c, err.Error())
		return
	}
	cfg, err := h.collateral.SetForBranch(c.Request.Context(), branchID, raw, nil)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCollateralConfig) {
			respondValidationError(c, err.Error())
			return
		}
		respondInternalError(c)
		return
	}
	h.audit.Record(c.Request.Context(), audit.AuditEvent{
		BranchID:     branchID,
		ResourceType: audit.ResourceBranch,
		ResourceID:   audit.IDStr(branchID),
		Action:       audit.ActionBranchSettingsUpdate,
		Result:       audit.ResultSuccess,
		ActorType:    audit.ActorTypeStaff,
		ActorID:      audit.IDStr(sess.StaffID),
		RiskLevel:    audit.RiskLow,
		Metadata:     map[string]any{"resource": "collateral", "format": cfg.Format},
	})
	c.JSON(http.StatusOK, gin.H{"collateral": cfg})
}
