package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/config"
	dbPkg "github.com/Mohith1612/qr-dining/internal/db"
	dbsqlc "github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/server"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/Mohith1612/qr-dining/internal/worker"
)

func main() {
	// 1. Load configuration from environment.
	cfg, err := config.Load()
	if err != nil {
		// zerolog not yet initialized — use plain stderr.
		os.Stderr.WriteString("fatal: load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	// 2. Initialize structured logger.
	logger := observability.Setup(cfg.Log)

	// 3. Root context — cancelled on SIGINT/SIGTERM for clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3b. Tracing (no-op unless OTEL_ENABLED=true).
	otelShutdown, err := observability.SetupTracing(ctx, cfg.OTel, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("otel init failed; continuing without tracing")
		otelShutdown = func(context.Context) error { return nil }
	}

	// 4. Connect to PostgreSQL.
	db, err := dbPkg.NewPool(ctx, cfg.DB)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect to postgres")
	}
	defer db.Close()
	logger.Info().Msg("postgres connected")

	// 5. Run pending database migrations before accepting traffic.
	if err := dbPkg.RunMigrations(db); err != nil {
		logger.Fatal().Err(err).Msg("run migrations")
	}
	logger.Info().Msg("migrations applied")

	// 6. Connect to Redis.
	redisClient, err := redisPkg.NewClient(ctx, cfg.Redis)
	if err != nil {
		logger.Fatal().Err(err).Msg("connect to redis")
	}
	defer redisClient.Close()
	logger.Info().Msg("redis connected")

	// 7. Initialize observability metrics.
	metrics := observability.NewMetrics()

	// 8. Initialize Redis helpers.
	pubsub := redisPkg.NewPubSub(redisClient, logger, metrics)
	presence := redisPkg.NewPresence(redisClient)
	presence.SetHostAbsenceGrace(cfg.Presence.HostAbsenceGrace)

	// 9. Initialize event publisher.
	publisher := events.NewPublisher(pubsub, logger)
	publisher.SetMetrics(metrics)

	// 10. Initialize WebSocket Hub.
	hub := ws.NewHub(pubsub, metrics, logger, cfg.CORS.AllowedOrigins)

	// 11. Initialize repository layer.
	repos := repository.New(db, logger)
	publisher.SetEventStore(repos)
	auditWriter := audit.NewWriter(dbsqlc.New(db), cfg.FeatureFlags.AuditLogV2Enabled, logger, metrics.AuditWriteFailuresTotal)

	// 12. Initialize HTTP server with all dependencies.
	srv := server.New(cfg, db, redisClient, hub, metrics, logger, repos, publisher)

	// 13. Start WebSocket Hub in background.
	go hub.Run(ctx)
	logger.Info().Msg("websocket hub started")

	// 14. Start background workers.
	wq := &workerQuerier{repos: repos}
	w := worker.New(db, wq, redisClient, publisher, presence, metrics, auditWriter, cfg.Worker.Region, logger)
	w.SetSessionIdleGrace(cfg.Worker.SessionIdleGrace)
	go w.RunStaleSessionCleaner(ctx, cfg.Worker.StaleSessionInterval)
	go w.RunSessionExpiryWarner(ctx, 5*time.Minute)
	go w.RunPresenceExpiry(ctx, cfg.Worker.PresenceExpiryInterval)
	go w.RunSessionTableReconciler(ctx, cfg.Worker.SessionReconcileInterval)
	go w.RunReactivationPipeline(ctx, cfg.Worker.PresenceExpiryInterval, cfg.Worker.SessionPresenceGrace, cfg.Worker.SessionReactivationWindow)
	go w.RunPaymentPendingEscalation(ctx, cfg.Worker.PaymentPendingEscalationInterval, cfg.Worker.PaymentPendingWarnAfter, cfg.Worker.PaymentPendingCriticalAfter)
	go w.RunBillingReconciliation(ctx, cfg.Worker.BillingReconciliationInterval)

	// 14b. Poll DB pool stats every 30s and export to Prometheus.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stat := db.Stat()
				metrics.DBPoolTotalConns.Set(float64(stat.TotalConns()))
				metrics.DBPoolIdleConns.Set(float64(stat.IdleConns()))
				metrics.DBPoolAcquiredConns.Set(float64(stat.AcquiredConns()))
				metrics.DBPoolAcquireCount.Add(float64(stat.AcquireCount()))
			}
		}
	}()

	// 15. Start HTTP server — blocks until shutdown.
	if err := srv.Start(ctx); err != nil {
		logger.Error().Err(err).Msg("server error")
	}

	otelCtx, cancelOTel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelOTel()
	if err := otelShutdown(otelCtx); err != nil {
		logger.Warn().Err(err).Msg("otel shutdown failed")
	}

	logger.Info().Msg("shutdown complete")
}
