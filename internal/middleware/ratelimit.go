package middleware

import (
	"fmt"
	"net/http"

	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/gin-gonic/gin"
)

// RateLimit enforces a per-IP fixed-window rate limit using Redis.
// On Redis failure, the middleware fails open to avoid blocking legitimate requests.
func RateLimit(rl *redisPkg.RateLimiter, limitPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, remaining, err := rl.Allow(c.Request.Context(), ip, limitPerMinute)
		if err != nil {
			// Redis error: fail open.
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Header("X-RateLimit-Remaining", itoa(remaining))
		c.Next()
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
