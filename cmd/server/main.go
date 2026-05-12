package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mohith1612/qr-dining/internal/config"
	dbPkg "github.com/Mohith1612/qr-dining/internal/db"
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
	pubsub := redisPkg.NewPubSub(redisClient, logger)
	presence := redisPkg.NewPresence(redisClient)

	// 9. Initialize event publisher.
	publisher := events.NewPublisher(pubsub, logger)

	// 10. Initialize WebSocket Hub.
	hub := ws.NewHub(pubsub, metrics, logger, cfg.CORS.AllowedOrigins)

	// 11. Initialize repository layer.
	repos := repository.New(db, logger)

	// 12. Initialize HTTP server with all dependencies.
	srv := server.New(cfg, db, redisClient, hub, metrics, logger, repos, publisher)

	// 13. Start WebSocket Hub in background.
	go hub.Run(ctx)
	logger.Info().Msg("websocket hub started")

	// 14. Start background workers.
	wq := &workerQuerier{repos: repos}
	w := worker.New(db, wq, redisClient, publisher, presence, metrics, logger)
	go w.RunStaleSessionCleaner(ctx, cfg.Worker.StaleSessionInterval)
	go w.RunPresenceExpiry(ctx, cfg.Worker.StaleSessionInterval)

	// 15. Start HTTP server — blocks until shutdown.
	if err := srv.Start(ctx); err != nil {
		logger.Error().Err(err).Msg("server error")
	}

	logger.Info().Msg("shutdown complete")
}
