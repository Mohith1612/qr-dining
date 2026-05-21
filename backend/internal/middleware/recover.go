package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Recover catches panics, logs them with a stack trace, and returns 500.
// The server continues running — one bad request cannot bring down the process.
func Recover(base zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log := GetLogger(c)
				if log.GetLevel() == zerolog.Disabled {
					log = base
				}
				log.Error().
					Interface("panic", r).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
