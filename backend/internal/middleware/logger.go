package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

const LoggerKey = "logger"

// Logger attaches a request-scoped zerolog logger to the context and emits
// a structured access log line after each request completes.
func Logger(base zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID, _ := c.Get(RequestIDKey)
		logCtx := base.With().Str("request_id", requestID.(string))
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
			logCtx = logCtx.Str("trace_id", sc.TraceID().String()).Str("span_id", sc.SpanID().String())
		}
		log := logCtx.Logger()

		c.Set(LoggerKey, log)
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		event := log.Info()
		if status >= 500 {
			event = log.Error()
		} else if status >= 400 {
			event = log.Warn()
		}

		if staff, ok := GetStaffSession(c); ok {
			event = event.
				Str("actor_type", "staff").
				Int64("actor_id", staff.StaffID).
				Int64("organization_id", staff.OrganizationID).
				Int64("branch_id", staff.BranchID)
		} else if platform, ok := GetPlatformSession(c); ok {
			event = event.
				Str("actor_type", "platform_user").
				Int64("actor_id", platform.PlatformUserID)
		} else if organizationID, ok := GetTenantOrganizationID(c); ok {
			event = event.Int64("organization_id", organizationID)
		}

		event.
			Str("method", c.Request.Method).
			Str("route", c.FullPath()).
			Str("action", c.Request.Method+" "+c.FullPath()).
			Str("client_ip", c.ClientIP()).
			Int("status", status).
			Dur("latency_ms", latency).
			Int("bytes", c.Writer.Size()).
			Msg("request")
	}
}

// GetLogger retrieves the request-scoped logger from the Gin context.
func GetLogger(c *gin.Context) zerolog.Logger {
	if l, exists := c.Get(LoggerKey); exists {
		return l.(zerolog.Logger)
	}
	return zerolog.Nop()
}
