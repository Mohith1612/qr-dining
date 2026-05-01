package middleware

import (
	"strconv"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/gin-gonic/gin"
)

// Metrics records Prometheus HTTP request counters and latency histograms.
func Metrics(m *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
