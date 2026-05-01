package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const LoggerKey = "logger"

// Logger attaches a request-scoped zerolog logger to the context and emits
// a structured access log line after each request completes.
func Logger(base zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID, _ := c.Get(RequestIDKey)
		log := base.With().
			Str("request_id", requestID.(string)).
			Logger()

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

		event.
			Str("method", c.Request.Method).
			Str("path", c.FullPath()).
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
