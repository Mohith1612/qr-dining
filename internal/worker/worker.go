package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// StaleSession is the minimal projection of a session needed by the cleaner.
type StaleSession struct {
	ID      uuid.UUID
	TableID int64
}

// Querier abstracts the DB queries the worker needs.
// Implemented by *sqlc.Queries after code generation; interface allows compilation before that.
type Querier interface {
	ListStaleSessions(ctx context.Context, interval string) ([]StaleSession, error)
	AbandonStaleSession(ctx context.Context, id uuid.UUID) error
}

// Worker runs lightweight background maintenance goroutines.
// NOT a distributed job queue — simple goroutines coordinated via Redis locks
// to prevent duplicate runs when multiple instances are deployed.
type Worker struct {
	db        *pgxpool.Pool
	queries   Querier
	redis     *goredis.Client
	publisher *events.Publisher
	presence  *redisPkg.Presence
	metrics   *observability.Metrics
	logger    zerolog.Logger
}

func New(
	db *pgxpool.Pool,
	queries Querier,
	redis *goredis.Client,
	publisher *events.Publisher,
	presence *redisPkg.Presence,
	metrics *observability.Metrics,
	logger zerolog.Logger,
) *Worker {
	return &Worker{
		db:        db,
		queries:   queries,
		redis:     redis,
		publisher: publisher,
		presence:  presence,
		metrics:   metrics,
		logger:    logger.With().Str("component", "worker").Logger(),
	}
}

// RunStaleSessionCleaner marks sessions abandoned after they've been active
// without a placed order or presence heartbeat for longer than staleDuration.
// Runs on the given interval and skips if another instance holds the Redis lock.
func (w *Worker) RunStaleSessionCleaner(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "stale_session_cleaner", 270*time.Second, func() {
				w.cleanStaleSessions(ctx)
			})
		}
	}
}

// RunPresenceExpiry scans active sessions and publishes PARTICIPANT_LEFT for
// participants whose last heartbeat has expired. Presence itself auto-expires
// via Redis TTL; this worker ensures WebSocket clients get a notification.
func (w *Worker) RunPresenceExpiry(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "presence_expiry", 50*time.Second, func() {
				// Presence TTL is managed by Redis itself; this is a no-op notification hook
				// for future implementation when participant-left events are needed on expiry.
				w.logger.Debug().Msg("presence expiry worker tick")
			})
		}
	}
}

func (w *Worker) cleanStaleSessions(ctx context.Context) {
	const staleAfter = "2 hours"
	sessions, err := w.queries.ListStaleSessions(ctx, staleAfter)
	if err != nil {
		w.logger.Error().Err(err).Msg("list stale sessions")
		w.metrics.WorkerRunsTotal.WithLabelValues("stale_session_cleaner", "error").Inc()
		return
	}

	abandoned := 0
	for i := range sessions {
		s := &sessions[i]
		if err := w.queries.AbandonStaleSession(ctx, s.ID); err != nil {
			w.logger.Error().Err(err).Str("session_id", s.ID.String()).Msg("abandon stale session")
			continue
		}
		w.publisher.SessionClosed(ctx, s.ID, map[string]string{"reason": "stale"})
		abandoned++
	}

	if abandoned > 0 {
		w.logger.Info().Int("abandoned", abandoned).Msg("stale session cleanup")
	}
	w.metrics.WorkerRunsTotal.WithLabelValues("stale_session_cleaner", "ok").Inc()
}

// runWithLock acquires a Redis NX lock before executing fn.
// If the lock is already held (another instance is running), fn is skipped.
func (w *Worker) runWithLock(ctx context.Context, name string, lockTTL time.Duration, fn func()) {
	key := fmt.Sprintf("worker:%s:lock", name)
	acquired, err := w.redis.SetNX(ctx, key, "1", lockTTL).Result()
	if err != nil || !acquired {
		return
	}
	defer w.redis.Del(ctx, key)
	fn()
}
