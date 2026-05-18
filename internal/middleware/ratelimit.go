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
	return rateLimitHandler(rl, "", limitPerMinute)
}

// RateLimitStrict applies a tighter limit scoped to a named prefix (e.g. "auth").
// Use for sensitive endpoints such as staff authentication to prevent brute force.
func RateLimitStrict(rl *redisPkg.RateLimiter, prefix string, limitPerMinute int) gin.HandlerFunc {
	return rateLimitHandler(rl, prefix, limitPerMinute)
}

func rateLimitHandler(rl *redisPkg.RateLimiter, prefix string, limitPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, remaining, err := rl.AllowWithPrefix(c.Request.Context(), prefix, ip, limitPerMinute)
		if err != nil {
			// Redis error: fail open.
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]string{
				"code":    "RATE_LIMITED",
				"message": "rate limit exceeded",
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
