package middleware

import (
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const StaffSessionKey = "staff_session"

// StaffAuth validates the Bearer token from the Authorization header against Redis.
// On success, the StaffSession is stored in the Gin context under StaffSessionKey.
func StaffAuth(staffSvc *services.StaffService, logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "missing or invalid Authorization header",
			})
			return
		}

		session, err := staffSvc.ValidateToken(c.Request.Context(), token)
		if err != nil {
			prefix := token
			if len(prefix) > 8 {
				prefix = prefix[:8]
			}
			logger.Warn().
				Str("token_prefix", prefix).
				Str("ip", c.ClientIP()).
				Msg("staff auth rejected: invalid or expired token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "invalid or expired token",
			})
			return
		}

		c.Set(StaffSessionKey, session)
		c.Next()
	}
}

// GetStaffSession retrieves the authenticated StaffSession from context.
func GetStaffSession(c *gin.Context) (services.StaffSession, bool) {
	v, exists := c.Get(StaffSessionKey)
	if !exists {
		return services.StaffSession{}, false
	}
	s, ok := v.(services.StaffSession)
	return s, ok
}
