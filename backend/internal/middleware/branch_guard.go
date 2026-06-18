package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
)

// BranchTenantGuard is a Gin middleware that enforces tenant ownership of the branch
// identified by the `:id` path parameter. It is applied to route groups that expose
// branch-scoped resources (e.g. `/branches/:id/...`).
//
// When tenant enforcement is disabled (BASE_DOMAIN unset), this middleware is a no-op.
// When enforcement is active, it verifies branch tenant ownership, returning 403 otherwise.
func BranchTenantGuard(repos *repository.Repos, organizationsEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantRestaurantID, hasTenant := GetTenantRestaurantID(c)
		if !hasTenant {
			c.Next()
			return
		}

		rawID := c.Param("id")
		branchID, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil {
			// Not a branch-id route or malformed — let the handler deal with it.
			c.Next()
			return
		}

		if organizationsEnabled {
			tenantOrganizationID, ok := GetTenantOrganizationID(c)
			if !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    "FORBIDDEN",
					"message": "tenant organization not resolved",
				})
				return
			}
			organization, err := repos.GetOrganizationByBranchID(c.Request.Context(), branchID)
			if err != nil {
				if errors.Is(err, domain.ErrTenantNotFound) {
					c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
						"code":    "BRANCH_NOT_FOUND",
						"message": "branch not found",
					})
					return
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "internal server error",
				})
				return
			}
			if organization.ID != tenantOrganizationID {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    "FORBIDDEN",
					"message": "branch does not belong to this tenant",
				})
				return
			}
			c.Next()
			return
		}

		restaurant, err := repos.GetRestaurantByBranchID(c.Request.Context(), branchID)
		if err != nil {
			if errors.Is(err, domain.ErrTenantNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"code":    "BRANCH_NOT_FOUND",
					"message": "branch not found",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "internal server error",
			})
			return
		}

		if restaurant.ID != tenantRestaurantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "branch does not belong to this tenant",
			})
			return
		}

		c.Next()
	}
}
