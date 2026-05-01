package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/handlers"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Server owns the HTTP server and all wired dependencies.
type Server struct {
	cfg    *config.Config
	router *gin.Engine
	http   *http.Server
	logger zerolog.Logger
}

// New constructs the Gin engine with all middleware and route groups.
func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	redis *goredis.Client,
	hub *ws.Hub,
	metrics *observability.Metrics,
	logger zerolog.Logger,
) *Server {
	gin.SetMode(cfg.Server.GinMode)

	r := gin.New()

	// SetTrustedProxies limits X-Forwarded-For trust to the nginx proxy network.
	_ = r.SetTrustedProxies(cfg.Server.TrustedProxies)

	// ── Middleware stack (applied in order) ──────────────────────────────────
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Metrics(metrics))
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))

	rateLimiter := redisPkg.NewRateLimiter(redis)

	// ── Handlers ─────────────────────────────────────────────────────────────
	health := handlers.NewHealthHandler(db, redis)

	// ── Routes ───────────────────────────────────────────────────────────────

	// Infrastructure — no rate limit, no auth.
	r.GET("/health", health.Health)
	r.GET("/readyz", health.Readiness)
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	// Public API — rate limited.
	api := r.Group("/")
	api.Use(middleware.RateLimit(rateLimiter, cfg.Server.RateLimitRPM))

	// Session lifecycle
	api.POST("/sessions", placeholder("create session"))
	api.GET("/sessions/:id", placeholder("get session"))
	api.DELETE("/sessions/:id", placeholder("close session"))
	api.POST("/sessions/:id/join", placeholder("join session"))

	// Cart
	api.GET("/sessions/:id/cart", placeholder("get cart"))
	api.POST("/sessions/:id/cart/items", placeholder("add cart item"))
	api.DELETE("/sessions/:id/cart/items/:item_id", placeholder("remove cart item"))

	// Orders
	api.POST("/sessions/:id/orders", placeholder("place order"))
	api.GET("/sessions/:id/orders", placeholder("list orders"))
	api.PATCH("/orders/:id/status", placeholder("update order status"))

	// Assistance
	api.POST("/sessions/:id/assist", placeholder("request assistance"))
	api.PATCH("/assist/:id/ack", placeholder("acknowledge assistance"))
	api.PATCH("/assist/:id/resolve", placeholder("resolve assistance"))

	// Payments
	api.POST("/sessions/:id/payments", placeholder("initiate payment"))
	api.POST("/webhooks/payments/:provider", placeholder("payment webhook"))

	// Menu & tables
	api.GET("/branches/:id/menu", placeholder("get menu"))
	api.GET("/tables/by-qr/:token", placeholder("get table by QR"))

	// Staff
	api.POST("/staff/auth", placeholder("staff auth"))

	// WebSocket
	r.GET("/ws", placeholder("websocket upgrade"))

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return &Server{
		cfg:    cfg,
		router: r,
		http:   httpServer,
		logger: logger,
	}
}

// Start begins listening and blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info().Int("port", s.cfg.Server.Port).Msg("HTTP server starting")

	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info().Msg("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}

// placeholder returns a stub handler for routes not yet implemented.
func placeholder(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented", "route": name})
	}
}
