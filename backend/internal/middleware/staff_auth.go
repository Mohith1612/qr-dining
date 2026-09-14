package middleware

import (
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	StaffSessionKey = "staff_session"
	StaffTokenKey   = "staff_token"
	StaffCookieName = "qrd_staff_session"
	StaffCookiePath = "/"
)

// StaffAuth validates a staff session token from EITHER the Authorization
// Bearer header OR a HttpOnly session cookie. The cookie path lets the
// frontend migrate off localStorage incrementally — both transports stay
// supported until the rollout is complete.
func StaffAuth(staffSvc *services.StaffService, logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := staffTokenFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "missing or invalid staff credentials",
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
		c.Set(StaffTokenKey, token)
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

// GetStaffToken retrieves the raw token the caller authenticated with. Logout
// needs it to address the right Redis key; nothing else should.
func GetStaffToken(c *gin.Context) (string, bool) {
	v, exists := c.Get(StaffTokenKey)
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// staffTokenFromRequest prefers the Authorization Bearer header for
// backwards compatibility with existing frontend clients, then falls back to
// the HttpOnly cookie. Both transports point at the same Redis/DB session
// row, so honoring either is safe.
func staffTokenFromRequest(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok && token != "" {
		return token
	}
	if cookie, err := c.Cookie(StaffCookieName); err == nil && cookie != "" {
		return cookie
	}
	return ""
}
