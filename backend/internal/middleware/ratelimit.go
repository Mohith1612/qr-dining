package middleware

import (
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/gin-gonic/gin"
)

// rateLimiterUnavailableMetric is set once at server bootstrap so this package
// can report fail-closed denials to operators. nil-safe.
var rateLimiterUnavailableMetric *observability.Metrics

// SetRateLimitMetrics wires observability into the rate-limit middleware.
func SetRateLimitMetrics(m *observability.Metrics) { rateLimiterUnavailableMetric = m }

// RateLimit enforces a per-IP fixed-window rate limit using Redis.
// On Redis failure, the middleware fails open to avoid blocking legitimate requests.
func RateLimit(rl *redisPkg.RateLimiter, limitPerMinute int) gin.HandlerFunc {
	return rateLimitHandler(rl, "", limitPerMinute, false, "")
}

// RateLimitStrict applies a tighter limit scoped to a named prefix (e.g. "auth").
// Use for sensitive endpoints such as staff authentication to prevent brute force.
func RateLimitStrict(rl *redisPkg.RateLimiter, prefix string, limitPerMinute int) gin.HandlerFunc {
	return rateLimitHandler(rl, prefix, limitPerMinute, false, "")
}

// RateLimitSensitive fails CLOSED when the rate-limit backend is unreachable —
// the request is rejected with 503 RATE_LIMITER_UNAVAILABLE instead of being
// passed through. Use for endpoints where a Redis outage must not weaken
// brute-force protection: staff auth, platform auth, payment initiation,
// webhook receipt, WS ticket issuance.
func RateLimitSensitive(rl *redisPkg.RateLimiter, surface string, limitPerMinute int) gin.HandlerFunc {
	return rateLimitHandler(rl, surface, limitPerMinute, true, surface)
}

// RateLimitByKey limits by a caller-supplied key (e.g. session ID, participant
// ID) instead of client IP. Useful for surfaces where IP is shared (NAT) but
// the actor is identifiable from the request (signed token claims, session id
// in path). Fails open like RateLimit.
func RateLimitByKey(rl *redisPkg.RateLimiter, prefix string, limitPerMinute int, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}
		allowed, remaining, err := rl.AllowWithPrefix(c.Request.Context(), prefix, key, limitPerMinute)
		if err != nil {
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

func rateLimitHandler(rl *redisPkg.RateLimiter, prefix string, limitPerMinute int, failClosed bool, surface string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, remaining, err := rl.AllowWithPrefix(c.Request.Context(), prefix, ip, limitPerMinute)
		if err != nil {
			if failClosed {
				if rateLimiterUnavailableMetric != nil && rateLimiterUnavailableMetric.RateLimiterUnavailableTotal != nil {
					rateLimiterUnavailableMetric.RateLimiterUnavailableTotal.WithLabelValues(surface).Inc()
				}
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, map[string]string{
					"code":    "RATE_LIMITER_UNAVAILABLE",
					"message": "rate-limit backend unavailable; please retry shortly",
				})
				return
			}
			// Fail open: only acceptable for non-sensitive surfaces.
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
