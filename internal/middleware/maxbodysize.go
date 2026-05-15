package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodySize rejects requests whose body exceeds maxBytes.
// Uses http.MaxBytesReader so the connection is not fully consumed on oversized uploads.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
