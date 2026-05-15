package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/handlers"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
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

// New constructs the Gin engine with all middleware, services, and route groups.
func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	redis *goredis.Client,
	hub *ws.Hub,
	metrics *observability.Metrics,
	logger zerolog.Logger,
	repos *repository.Repos,
	publisher *events.Publisher,
) *Server {
	gin.SetMode(cfg.Server.GinMode)

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		logger.Fatal().Err(err).Strs("trusted_proxies", cfg.Server.TrustedProxies).Msg("invalid TRUSTED_PROXIES configuration")
	}

	// ── Middleware stack ─────────────────────────────────────────────────────
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Metrics(metrics))
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MB request body limit

	rateLimiter := redisPkg.NewRateLimiter(redis)
	cache := redisPkg.NewCache(redis, metrics.CacheHitsTotal, metrics.CacheMissesTotal)
	presence := redisPkg.NewPresence(redis)

	// ── Services ─────────────────────────────────────────────────────────────
	sessionSvc := services.NewSessionService(repos, publisher)
	participantSvc := services.NewParticipantService(repos, publisher, presence)
	cartSvc := services.NewCartService(repos, publisher)
	orderSvc := services.NewOrderService(repos, publisher)
	assistanceSvc := services.NewAssistanceService(repos, publisher)
	menuSvc := services.NewMenuService(repos, cache)
	staffSvc := services.NewStaffService(repos, cache)
	paymentSvc := services.NewPaymentService(repos, publisher)

	// ── Handlers ─────────────────────────────────────────────────────────────
	health := handlers.NewHealthHandler(db, redis)
	sessionH := handlers.NewSessionHandler(sessionSvc)
	cartH := handlers.NewCartHandler(cartSvc)
	orderH := handlers.NewOrderHandler(orderSvc)
	assistanceH := handlers.NewAssistanceHandler(assistanceSvc)
	menuH := handlers.NewMenuHandler(menuSvc)
	staffH := handlers.NewStaffHandler(staffSvc)
	paymentH := handlers.NewPaymentHandler(paymentSvc)
	wsH := handlers.NewWSHandler(hub, repos)

	_ = participantSvc // used by ws handler indirectly

	// ── Routes ───────────────────────────────────────────────────────────────

	// Infrastructure — no auth, no rate limit.
	r.GET("/health", health.Health)
	r.GET("/readyz", health.Readiness)
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	// Public API — rate limited.
	api := r.Group("/")
	api.Use(middleware.RateLimit(rateLimiter, cfg.Server.RateLimitRPM))

	// Session lifecycle
	api.POST("/sessions", sessionH.Create)
	api.GET("/sessions/:id", sessionH.Get)
	api.DELETE("/sessions/:id", sessionH.Close)
	api.POST("/sessions/:id/join", sessionH.Join)

	// Cart
	api.GET("/sessions/:id/cart", cartH.GetCart)
	api.POST("/sessions/:id/cart/items", cartH.AddItem)
	api.DELETE("/sessions/:id/cart/items/:item_id", cartH.RemoveItem)

	// Orders
	api.POST("/sessions/:id/orders", orderH.PlaceOrder)
	api.GET("/sessions/:id/orders", orderH.ListOrders)

	// Assistance
	api.POST("/sessions/:id/assist", assistanceH.Request)

	// Payments
	api.POST("/sessions/:id/payments", paymentH.InitiatePayment)
	api.POST("/webhooks/payments/:provider", paymentH.Webhook)

	// Menu & tables — public
	api.GET("/branches/:id/menu", menuH.GetMenu)
	api.GET("/tables/by-qr/:token", menuH.GetTableByQR)

	// Staff auth — strict 10 RPM limit to prevent PIN brute force.
	authGroup := r.Group("/")
	authGroup.Use(middleware.RateLimitStrict(rateLimiter, "auth", 10))
	authGroup.POST("/staff/auth", staffH.Authenticate)

	// Staff-protected routes (require valid staff token).
	staffAPI := r.Group("/")
	staffAPI.Use(middleware.RateLimit(rateLimiter, cfg.Server.RateLimitRPM))
	staffAPI.Use(middleware.StaffAuth(staffSvc, logger))

	staffAPI.PATCH("/orders/:id/status", orderH.UpdateStatus)
	staffAPI.PATCH("/assist/:id/ack", assistanceH.Acknowledge)
	staffAPI.PATCH("/assist/:id/resolve", assistanceH.Resolve)

	// Staff dashboard — branch-scoped operational views.
	staffAPI.GET("/branches/:id/orders/active", orderH.ListActiveForBranch)
	staffAPI.GET("/branches/:id/sessions/active", sessionH.ListActiveForBranch)
	staffAPI.GET("/branches/:id/assist/active", assistanceH.ListActiveForBranch)

	// WebSocket
	r.GET("/ws", wsH.Upgrade)

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
