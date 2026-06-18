package audit

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// requestIDKey matches middleware.RequestIDKey — duplicated here to avoid import cycle.
const requestIDKey = "request_id"

type auditCtxKey struct{}

// AuditRequestContext holds per-request metadata extracted by Middleware.
type AuditRequestContext struct {
	IP            string
	UserAgent     string
	RequestID     string
	CorrelationID string
	Source        SourceType
}

// Middleware extracts IP, User-Agent, RequestID, X-Correlation-ID, and inferred source
// from the Gin context and stores them in the request context for Writer.Record to use.
// Must be registered after middleware.RequestID().
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqCtx := AuditRequestContext{
			IP:            c.ClientIP(),
			UserAgent:     c.GetHeader("User-Agent"),
			RequestID:     c.GetString(requestIDKey),
			CorrelationID: c.GetHeader("X-Correlation-ID"),
			Source:        inferSource(c),
		}
		ctx := context.WithValue(c.Request.Context(), auditCtxKey{}, reqCtx)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// FromContext retrieves the AuditRequestContext set by Middleware.
// Returns a zero-value struct if not present (safe for non-HTTP code paths).
func FromContext(ctx context.Context) AuditRequestContext {
	v, _ := ctx.Value(auditCtxKey{}).(AuditRequestContext)
	return v
}

func inferSource(c *gin.Context) SourceType {
	if explicit := c.GetHeader("X-Source"); explicit != "" {
		switch SourceType(strings.ToLower(explicit)) {
		case SourceWeb, SourceMobile, SourcePWA, SourceAPI, SourceWebhook, SourceSystem:
			return SourceType(strings.ToLower(explicit))
		}
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	if strings.Contains(ua, "flutter") || strings.Contains(ua, "dart") ||
		strings.Contains(ua, "react-native") || strings.Contains(ua, "expo") {
		return SourceMobile
	}
	if strings.Contains(ua, "pwa") || strings.Contains(ua, "standalone") {
		return SourcePWA
	}
	return SourceWeb
}
