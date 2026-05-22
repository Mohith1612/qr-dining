package middleware

import (
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const TenantRestaurantIDKey = "tenant_restaurant_id"
const TenantRestaurantSlugKey = "tenant_restaurant_slug"

// TenantMiddleware extracts the restaurant slug from the Host subdomain, resolves it to a
// restaurant record, and injects the restaurant_id into the Gin context.
//
// When baseDomain is empty (local development), the middleware is a no-op — all requests
// proceed without tenant enforcement. When baseDomain is set (e.g. "dining.example.com"),
// a request to "olive.dining.example.com" resolves slug "olive".
func TenantMiddleware(repos *repository.Repos, baseDomain string, logger zerolog.Logger) gin.HandlerFunc {
	if baseDomain == "" {
		// No-op in local dev — tenant enforcement disabled.
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		slug := extractSlug(c.Request.Host, baseDomain)
		if slug == "" {
			// Host does not match expected subdomain pattern — allow through (e.g. health checks).
			c.Next()
			return
		}

		restaurant, err := repos.GetRestaurantBySlug(c.Request.Context(), slug)
		if err != nil {
			if err == domain.ErrTenantNotFound {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"code":    "TENANT_NOT_FOUND",
					"message": "restaurant not found for this subdomain",
				})
				return
			}
			logger.Error().Err(err).Str("slug", slug).Msg("tenant resolution failed")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "internal server error",
			})
			return
		}

		c.Set(TenantRestaurantIDKey, restaurant.ID)
		c.Set(TenantRestaurantSlugKey, restaurant.Slug)

		// Enrich request-scoped logger with tenant context.
		if log, exists := c.Get(LoggerKey); exists {
			enriched := log.(zerolog.Logger).With().
				Str("tenant_slug", restaurant.Slug).
				Int64("restaurant_id", restaurant.ID).
				Logger()
			c.Set(LoggerKey, enriched)
		}

		c.Next()
	}
}

// GetTenantRestaurantID returns the resolved restaurant_id from the Gin context.
// Returns (0, false) when tenant enforcement is disabled (local dev / no BASE_DOMAIN).
func GetTenantRestaurantID(c *gin.Context) (int64, bool) {
	v, exists := c.Get(TenantRestaurantIDKey)
	if !exists {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// extractSlug strips the base domain from the host header and returns the subdomain slug.
// e.g. host="olive.dining.example.com", baseDomain="dining.example.com" → "olive"
// Returns "" if the host does not match the pattern.
func extractSlug(host, baseDomain string) string {
	// Strip port if present.
	if idx := strings.LastIndex(host, ":"); idx > strings.LastIndex(host, "]") {
		host = host[:idx]
	}
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	slug := strings.TrimSuffix(host, suffix)
	// Reject nested subdomains (e.g. "a.b.dining.example.com").
	if strings.Contains(slug, ".") {
		return ""
	}
	return slug
}
