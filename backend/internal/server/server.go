package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	dbsqlc "github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/handlers"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/storage"
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
	r.Use(audit.Middleware())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Metrics(metrics))
	r.Use(middleware.SecurityHeaders(cfg.Server.EnableHSTS))
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MB request body limit
	r.Use(middleware.TenantMiddleware(repos, cfg.Server.BaseDomain, cfg.FeatureFlags.TenancyOrganizationsEnabled, logger))

	rateLimiter := redisPkg.NewRateLimiter(redis)
	cache := redisPkg.NewCache(redis, metrics.CacheHitsTotal, metrics.CacheMissesTotal)
	presence := redisPkg.NewPresence(redis)
	wsTickets := redisPkg.NewWSTicketStore(redis)
	guestTokens := auth.NewGuestTokenService(cfg.Auth.GuestTokenSecret, cfg.Auth.GuestTokenTTL)
	authorizer := authz.NewEnforcingAuthorizer(cfg.FeatureFlags.AuthzCentralPolicyEnforce)
	handlers.SetHandlerMetrics(metrics)
	handlers.SetActiveFeatureFlags(cfg.FeatureFlags)
	handlers.SetActiveAuthConfig(cfg.Auth)
	handlers.SetActiveServerConfig(cfg.Server)
	middleware.SetRateLimitMetrics(metrics)
	middleware.SetTenantMetrics(metrics)

	// ── Services ─────────────────────────────────────────────────────────────
	sessionSvc := services.NewSessionService(repos, publisher, metrics, presence)
	participantSvc := services.NewParticipantService(repos, publisher, presence)
	cartSvc := services.NewCartService(repos, publisher)
	promoSvc := services.NewPromoService(repos)
	orderSvc := services.NewOrderService(repos, publisher, metrics, promoSvc)
	assistanceSvc := services.NewAssistanceService(repos, publisher)
	menuSvc := services.NewMenuService(repos, cache, publisher)
	lockoutStore := redisPkg.NewLockoutStore(redis)
	staffSvc := services.NewStaffService(repos, cache, logger)
	staffSvc.SetRequireSessionDBRow(cfg.FeatureFlags.AuthStaffSessionDBRequired)
	staffSvc.SetLockoutStore(lockoutStore)
	platformSvc := services.NewPlatformService(repos, cache, logger)
	platformSvc.SetLockoutStore(lockoutStore)
	platformSvc.SetMFAEncryptionKey(cfg.Auth.MFAEncryptionKey)
	paymentSvc := services.NewPaymentService(repos, publisher, metrics, sessionSvc, logger)
	// Host-controlled ordering: only the session host may submit orders or
	// initiate payment. The session service is the single host authority.
	orderSvc.SetHostAuthority(sessionSvc)
	paymentSvc.SetHostAuthority(sessionSvc)
	subSvc := services.NewSubscriptionService(repos)
	analyticsSvc := services.NewAnalyticsService(repos, subSvc, cache)
	entitlementSvc := services.NewEntitlementService(repos, subSvc, metrics)
	customerSvc := services.NewCustomerService(repos)

	// ── Audit writer ─────────────────────────────────────────────────────────
	auditWriter := audit.NewWriter(dbsqlc.New(db), cfg.FeatureFlags.AuditLogV2Enabled, logger, metrics.AuditWriteFailuresTotal)

	// ── Handlers ─────────────────────────────────────────────────────────────
	health := handlers.NewHealthHandler(db, redis)
	sessionH := handlers.NewSessionHandler(sessionSvc, repos, metrics, guestTokens, wsTickets, cfg.FeatureFlags, auditWriter)
	cartH := handlers.NewCartHandler(cartSvc, repos, metrics, guestTokens, cfg.FeatureFlags)
	orderH := handlers.NewOrderHandler(orderSvc, repos, metrics, guestTokens, cfg.FeatureFlags, authorizer, auditWriter)
	assistanceH := handlers.NewAssistanceHandler(assistanceSvc, repos, metrics, guestTokens, cfg.FeatureFlags, authorizer, auditWriter)
	menuH := handlers.NewMenuHandler(menuSvc)
	staffH := handlers.NewStaffHandler(staffSvc, repos, metrics, cfg.FeatureFlags, authorizer, auditWriter)
	platformH := handlers.NewPlatformHandler(repos, platformSvc, entitlementSvc, auditWriter)
	paymentH := handlers.NewPaymentHandler(paymentSvc, repos, guestTokens, cfg.FeatureFlags, cfg.Payment, authorizer, auditWriter)
	wsH := handlers.NewWSHandler(hub, repos, metrics, guestTokens, wsTickets, cfg.FeatureFlags)
	snapshotH := handlers.NewSnapshotHandler(sessionSvc, repos, guestTokens, cfg.FeatureFlags)
	menuAdminH := handlers.NewMenuAdminHandler(menuSvc, repos, authorizer, auditWriter)
	eventLogH := handlers.NewEventLogHandler(repos)
	tenantH := handlers.NewTenantHandler(repos)
	subH := handlers.NewSubscriptionHandler(repos, subSvc)
	analyticsH := handlers.NewAnalyticsHandler(analyticsSvc)
	orgH := handlers.NewOrganizationHandler(repos, analyticsSvc, cfg.FeatureFlags, authorizer, auditWriter)
	tableH := handlers.NewTableHandler(repos, auditWriter)
	branchH := handlers.NewBranchHandler(repos, auditWriter)
	customerH := handlers.NewCustomerHandler(customerSvc, repos, guestTokens, cfg.FeatureFlags, authorizer, auditWriter)
	uploadH := handlers.NewUploadHandler(storage.NewR2Client(cfg.R2), repos)
	billingH := handlers.NewBillingHandler(repos, guestTokens, cfg.FeatureFlags)
	promoH := handlers.NewPromoHandler(promoSvc, repos, guestTokens, cfg.FeatureFlags, authorizer, auditWriter)
	auditLogH := handlers.NewAuditLogHandler(repos, authorizer, auditWriter)

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
	api.POST("/sessions/:id/reactivate", sessionH.Reactivate)
	api.POST("/sessions/:id/ws-ticket",
		middleware.RateLimitSensitive(rateLimiter, "ws_ticket", 60),
		middleware.RateLimitByKey(rateLimiter, "ws_ticket_session", 12, func(c *gin.Context) string { return c.Param("id") }),
		sessionH.IssueWSTicket,
	)

	// Cart
	api.GET("/sessions/:id/cart", cartH.GetCart)
	api.POST("/sessions/:id/cart/items", cartH.AddItem)
	api.DELETE("/sessions/:id/cart/items/:item_id", cartH.RemoveItem)

	// Orders — per-session cap on placement curtails replay storms and
	// idempotency-key fuzzing. Falls back to global RPM if session id is
	// missing.
	api.POST("/sessions/:id/orders",
		middleware.RateLimitByKey(rateLimiter, "order_place_session", 12, func(c *gin.Context) string { return c.Param("id") }),
		orderH.PlaceOrder,
	)
	api.GET("/sessions/:id/orders", orderH.ListOrders)

	// Assistance — low cap because legitimate guests rarely tap "call waiter"
	// more than a handful of times per service.
	api.POST("/sessions/:id/assist",
		middleware.RateLimitByKey(rateLimiter, "assist_session", 6, func(c *gin.Context) string { return c.Param("id") }),
		assistanceH.Request,
	)

	// Payments — sensitive endpoints fail closed on rate-limit backend outage so
	// a Redis blip cannot weaken brute-force / replay protection. Also adds a
	// per-session limit on initiation so a stuck client cannot retry-spam.
	api.POST("/sessions/:id/payments",
		middleware.RateLimitSensitive(rateLimiter, "payment_init", 30),
		middleware.RateLimitByKey(rateLimiter, "payment_init_session", 6, func(c *gin.Context) string { return c.Param("id") }),
		paymentH.InitiatePayment,
	)
	api.GET("/sessions/:id/bill", billingH.GetBill)
	api.POST("/webhooks/payments/:provider",
		middleware.RateLimitSensitive(rateLimiter, "webhook", 200),
		paymentH.Webhook,
	)

	// Customer opt-in (guest, no auth)
	api.POST("/sessions/:id/customer", customerH.LinkCustomer)

	// Promo validation — public, rate-limited.
	api.POST("/sessions/:id/promos/validate", promoH.ValidatePromo)

	// Menu & tables — public (branch tenant-guarded when BASE_DOMAIN is set)
	branchPublicAPI := api.Group("/branches/:id")
	branchPublicAPI.Use(middleware.BranchTenantGuard(repos, cfg.FeatureFlags.TenancyOrganizationsEnabled))
	branchPublicAPI.GET("/menu", menuH.GetMenu)
	api.GET("/tables/by-qr/:token", menuH.GetTableByQR)

	// Reconnect reconciliation — full session state snapshot for WebSocket clients.
	api.GET("/sessions/:id/snapshot", snapshotH.GetSnapshot)

	// Tenant resolution — public, used by frontend to initialize context.
	api.GET("/tenants/by-slug/:slug", tenantH.GetBySlug)

	// Subscription plans — public.
	api.GET("/plans", subH.ListPlans)

	// Staff auth — strict 10 RPM limit to prevent PIN brute force. Fails closed
	// when Redis is unavailable so an infra blip cannot disable brute-force
	// protection on a credential endpoint.
	authGroup := r.Group("/")
	authGroup.Use(middleware.RateLimitSensitive(rateLimiter, "auth", cfg.Server.AuthRateLimitRPM))
	authGroup.POST("/staff/auth", staffH.Authenticate)
	authGroup.POST("/platform/auth", platformH.Authenticate)
	authGroup.POST("/platform/auth/mfa", platformH.CompleteMFA)

	// Platform-protected routes (separate trust domain; staff tokens are rejected).
	platformAPI := r.Group("/platform")
	platformAPI.Use(middleware.RateLimit(rateLimiter, cfg.Server.RateLimitRPM))
	platformAPI.Use(middleware.PlatformAuth(platformSvc, logger))
	platformAPI.POST("/auth/logout", platformH.Logout)
	platformAPI.POST("/mfa/enroll", platformH.BeginMFAEnrollment)
	platformAPI.POST("/mfa/confirm", platformH.ConfirmMFAEnrollment)
	platformAPI.POST("/mfa/disable", platformH.DisableMFA)
	platformAPI.GET("/users", platformH.ListUsers)
	platformAPI.GET("/users/:id", platformH.GetUser)
	platformAPI.GET("/organizations", platformH.ListOrganizations)
	platformAPI.POST("/organizations", platformH.CreateOrganization)
	platformAPI.GET("/organizations/:org_id", platformH.GetOrganization)
	platformAPI.PATCH("/organizations/:org_id", platformH.UpdateOrganization)
	platformAPI.GET("/organizations/:org_id/branches", platformH.ListBranches)
	platformAPI.POST("/organizations/:org_id/branches", platformH.CreateBranch)
	platformAPI.GET("/branches/:branch_id", platformH.GetBranch)
	platformAPI.GET("/support/search", platformH.SearchSupport)
	platformAPI.POST("/support/sessions", platformH.CreateSupportSession)
	platformAPI.GET("/support/sessions", platformH.ListSupportSessions)
	platformAPI.GET("/support/sessions/:id", platformH.GetSupportSession)
	platformAPI.GET("/audit", platformH.ListAudit)

	// Plan & entitlement governance (organization-level; resolve-only/shadow).
	platformAPI.GET("/entitlements", platformH.ListEntitlements)
	platformAPI.GET("/plans", platformH.ListPlatformPlans)
	platformAPI.POST("/plans", platformH.CreatePlan)
	platformAPI.PATCH("/plans/:plan_id", platformH.UpdatePlan)
	platformAPI.PUT("/plans/:plan_id/entitlements", platformH.SetPlanEntitlements)
	platformAPI.GET("/organizations/:org_id/entitlements", platformH.GetOrganizationEntitlements)
	platformAPI.PUT("/organizations/:org_id/plan", platformH.AssignOrganizationPlan)
	platformAPI.PUT("/organizations/:org_id/entitlements/:key", platformH.SetOrganizationEntitlementOverride)

	// Staff-protected routes (require valid staff token).
	staffAPI := r.Group("/")
	staffAPI.Use(middleware.RateLimit(rateLimiter, cfg.Server.RateLimitRPM))
	staffAPI.Use(middleware.StaffAuth(staffSvc, logger))

	staffAPI.POST("/staff/logout", staffH.Logout)
	staffAPI.PATCH("/orders/:id/status", orderH.UpdateStatus)
	staffAPI.PATCH("/payments/:id/settle", paymentH.Settle)
	staffAPI.PATCH("/assist/:id/ack", assistanceH.Acknowledge)
	staffAPI.PATCH("/assist/:id/resolve", assistanceH.Resolve)

	// Staff dashboard — branch-scoped operational views (tenant-guarded).
	branchStaffAPI := staffAPI.Group("/branches/:id")
	branchStaffAPI.Use(middleware.BranchTenantGuard(repos, cfg.FeatureFlags.TenancyOrganizationsEnabled))
	branchStaffAPI.GET("/orders/active", orderH.ListActiveForBranch)
	branchStaffAPI.GET("/sessions/active", sessionH.ListActiveForBranch)
	branchStaffAPI.GET("/assist/active", assistanceH.ListActiveForBranch)
	branchStaffAPI.GET("/payments", paymentH.ListPendingForBranch)
	branchStaffAPI.GET("/menu/full", menuAdminH.GetAdminMenu)
	branchStaffAPI.POST("/menu/categories", menuAdminH.CreateCategory)
	branchStaffAPI.POST("/menu/items", menuAdminH.CreateItem)
	branchStaffAPI.POST("/staff", staffH.CreateStaff)
	branchStaffAPI.GET("/events/recent", eventLogH.GetBranchRecentEvents)
	branchStaffAPI.GET("/analytics/top-items", analyticsH.GetTopItems)
	branchStaffAPI.GET("/analytics/busy-hours", analyticsH.GetBusyHours)
	branchStaffAPI.GET("/analytics/order-volume", analyticsH.GetOrderVolume)
	branchStaffAPI.GET("/tables", tableH.ListTables)
	branchStaffAPI.POST("/tables", tableH.CreateTable)
	branchStaffAPI.GET("", branchH.GetBranch)
	branchStaffAPI.PATCH("", branchH.UpdateBranch)
	branchStaffAPI.GET("/customers", customerH.SearchCustomers)

	branchStaffAPI.GET("/audit", auditLogH.GetBranchAuditLog)

	// Organization governance — staff authenticated, feature-flagged in handler.
	orgAPI := staffAPI.Group("/orgs/:org_id")
	orgAPI.GET("", orgH.GetOrganization)
	orgAPI.PATCH("", orgH.UpdateOrganization)
	orgAPI.GET("/branches", orgH.ListBranches)
	orgAPI.GET("/analytics/top-items", orgH.GetTopItems)
	orgAPI.GET("/analytics/busy-hours", orgH.GetBusyHours)
	orgAPI.GET("/analytics/order-volume", orgH.GetOrderVolume)
	orgAPI.GET("/audit", auditLogH.GetOrgAuditLog)

	// Promo management — staff protected.
	branchStaffAPI.GET("/promos", promoH.ListPromos)
	branchStaffAPI.POST("/promos", promoH.CreatePromo)
	branchStaffAPI.DELETE("/promos/:promo_id", promoH.DeactivatePromo)

	// Menu item updates — item-scoped, no branch param on path.
	staffAPI.PATCH("/menu/items/:id", menuAdminH.UpdateItem)
	staffAPI.DELETE("/menu/items/:id", menuAdminH.DeleteMenuItem)
	staffAPI.PATCH("/menu/items/:id/availability", menuAdminH.ToggleAvailability)
	staffAPI.PATCH("/menu/items/:id/featured", menuAdminH.ToggleFeatured)
	staffAPI.POST("/menu/items/:id/modifiers", menuAdminH.AddItemModifier)
	staffAPI.DELETE("/menu/categories/:id", menuAdminH.DeleteMenuCategory)
	staffAPI.PATCH("/menu/categories/:id", menuAdminH.UpdateMenuCategory)
	staffAPI.DELETE("/menu/modifiers/:id", menuAdminH.DeleteItemModifier)

	// Image upload presign — staff-protected.
	staffAPI.POST("/upload/menu-item-image", uploadH.PresignMenuItemImage)
	staffAPI.POST("/upload/restaurant-logo", uploadH.PresignRestaurantLogo)

	// Table QR token refresh — table-scoped; branch ownership verified in handler.
	staffAPI.PATCH("/tables/:id/qr-refresh", tableH.RefreshQR)

	// Staff management — owner only (role enforced in handler).
	staffAPI.PATCH("/staff/:id/pin", staffH.RotatePIN)
	staffAPI.PATCH("/staff/:id/deactivate", staffH.DeactivateStaff)

	// event_log read APIs — operational debugging and audit.
	staffAPI.GET("/sessions/:id/events", eventLogH.GetSessionEvents)

	// Subscription status — staff-protected.
	staffAPI.GET("/restaurants/:id/subscription", subH.GetSubscription)

	// Customer history and deletion — staff-protected.
	staffAPI.GET("/customers/:id/history", customerH.GetCustomerHistory)
	staffAPI.DELETE("/customers/:id", customerH.DeleteCustomer)

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
