package middleware

import (
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const PlatformSessionKey = "platform_session"
const PlatformTokenKey = "platform_token"

func PlatformAuth(platformSvc *services.PlatformService, logger zerolog.Logger) gin.HandlerFunc {
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

		session, err := platformSvc.ValidateToken(c.Request.Context(), token)
		if err != nil {
			prefix := token
			if len(prefix) > 8 {
				prefix = prefix[:8]
			}
			logger.Warn().
				Str("token_prefix", prefix).
				Str("ip", c.ClientIP()).
				Msg("platform auth rejected: invalid or expired token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "invalid or expired platform token",
			})
			return
		}

		c.Set(PlatformSessionKey, session)
		c.Set(PlatformTokenKey, token)
		c.Next()
	}
}

func GetPlatformSession(c *gin.Context) (services.PlatformSession, bool) {
	v, exists := c.Get(PlatformSessionKey)
	if !exists {
		return services.PlatformSession{}, false
	}
	s, ok := v.(services.PlatformSession)
	return s, ok
}

func GetPlatformToken(c *gin.Context) (string, bool) {
	v, exists := c.Get(PlatformTokenKey)
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
