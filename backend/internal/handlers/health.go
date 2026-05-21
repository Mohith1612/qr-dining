package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// HealthHandler holds dependencies for liveness and readiness checks.
type HealthHandler struct {
	db    *pgxpool.Pool
	redis *goredis.Client
}

func NewHealthHandler(db *pgxpool.Pool, redis *goredis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

// Health returns 200 immediately — proves the process is alive.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readiness checks downstream dependencies before accepting traffic.
// Returns 503 if PostgreSQL or Redis is unreachable.
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx := c.Request.Context()

	checks := gin.H{}
	ready := true

	if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = "unhealthy"
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy"
		ready = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"status": "ready", "checks": checks})
}
