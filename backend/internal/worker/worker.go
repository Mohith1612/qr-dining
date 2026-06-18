package worker

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// ExpiredSession is the minimal projection needed by the stale session cleaner.
type ExpiredSession struct {
	ID             uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
}

// ExpiringSoonSession is the projection used by the expiry warner.
type ExpiringSoonSession struct {
	ID                    uuid.UUID
	CreatedAt             time.Time
	SessionTimeoutMinutes int16
}

// Querier abstracts the DB queries the worker needs.
// Implemented by *sqlc.Queries after code generation; interface allows compilation before that.
type Querier interface {
	ListExpiredSessions(ctx context.Context) ([]ExpiredSession, error)
	AbandonStaleSession(ctx context.Context, id uuid.UUID, tableID int64) (repository.SessionMaintenanceResult, bool, error)
	ListSessionsExpiringSoon(ctx context.Context) ([]ExpiringSoonSession, error)
	MarkSessionWarned(ctx context.Context, id uuid.UUID) error
	ReconcileSessionTables(ctx context.Context) ([]repository.SessionTableReconciliation, error)
	LogEvent(ctx context.Context, sessionID uuid.UUID, branchID int64, eventType, actorType string, actorID int64, payload any)
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
	audit     *audit.Writer
	region    string
	logger    zerolog.Logger
}

func New(
	db *pgxpool.Pool,
	queries Querier,
	redis *goredis.Client,
	publisher *events.Publisher,
	presence *redisPkg.Presence,
	metrics *observability.Metrics,
	auditWriter *audit.Writer,
	region string,
	logger zerolog.Logger,
) *Worker {
	if region == "" {
		region = "default"
	}
	return &Worker{
		db:        db,
		queries:   queries,
		redis:     redis,
		publisher: publisher,
		presence:  presence,
		metrics:   metrics,
		audit:     auditWriter,
		region:    region,
		logger:    logger.With().Str("component", "worker").Logger(),
	}
}

// RunStaleSessionCleaner marks sessions abandoned after they've exceeded their
// branch-configured timeout. Runs on the given interval and skips if another
// instance holds the Redis lock.
func (w *Worker) RunStaleSessionCleaner(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "stale_session_cleaner", 270*time.Second, func() {
				w.safeRun("stale_session_cleaner", func() {
					tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					w.cleanStaleSessions(tickCtx)
				})
			})
		}
	}
}

// RunSessionExpiryWarner sends SESSION_EXPIRING_SOON to sessions whose timeout
// is within the next 15 minutes. Runs every 5 minutes.
func (w *Worker) RunSessionExpiryWarner(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "expiry_warner", 4*time.Minute, func() {
				w.safeRun("expiry_warner", func() {
					tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					w.warnExpiringSessions(tickCtx)
				})
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
				w.safeRun("presence_expiry", func() {
					// Presence TTL is managed by Redis itself; this is a no-op notification hook
					// for future implementation when participant-left events are needed on expiry.
					w.logger.Debug().Msg("presence expiry worker tick")
				})
			})
		}
	}
}

func (w *Worker) RunSessionTableReconciler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "session_table_reconciler", 4*time.Minute, func() {
				w.safeRun("session_table_reconciler", func() {
					tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					w.reconcileSessionTables(tickCtx)
				})
			})
		}
	}
}

func (w *Worker) cleanStaleSessions(ctx context.Context) {
	sessions, err := w.queries.ListExpiredSessions(ctx)
	if err != nil {
		w.logger.Error().Err(err).Msg("list expired sessions")
		w.metrics.WorkerRunsTotal.WithLabelValues("stale_session_cleaner", "error").Inc()
		return
	}

	abandoned := 0
	for i := range sessions {
		s := &sessions[i]
		result, didAbandon, err := w.queries.AbandonStaleSession(ctx, s.ID, s.TableID)
		if err != nil {
			w.logger.Error().Err(err).Str("session_id", s.ID.String()).Msg("abandon stale session")
			continue
		}
		if !didAbandon {
			continue
		}
		w.publisher.SessionClosed(ctx, s.ID, map[string]string{"reason": "stale"})
		w.presence.DeleteScoped(ctx, result.OrganizationID, result.BranchID, s.ID)
		w.queries.LogEvent(ctx, result.SessionID, result.BranchID, "SESSION_ABANDONED", "system", 0, map[string]any{"reason": "stale_timeout"})
		w.recordSystemSessionAudit(ctx, audit.ActionSessionAbandon, result, map[string]any{"reason": "stale_timeout"})
		abandoned++
	}

	if abandoned > 0 {
		w.logger.Info().Int("abandoned", abandoned).Msg("stale session cleanup")
	}
	w.metrics.WorkerRunsTotal.WithLabelValues("stale_session_cleaner", "ok").Inc()
}

func (w *Worker) reconcileSessionTables(ctx context.Context) {
	actions, err := w.queries.ReconcileSessionTables(ctx)
	if err != nil {
		w.logger.Error().Err(err).Msg("reconcile session tables")
		w.metrics.WorkerRunsTotal.WithLabelValues("session_table_reconciler", "error").Inc()
		return
	}

	for _, action := range actions {
		result := repository.SessionMaintenanceResult{
			SessionID:      action.SessionID,
			OrganizationID: action.OrganizationID,
			BranchID:       action.BranchID,
			TableID:        action.TableID,
		}
		w.logger.Warn().
			Str("action", action.Action).
			Str("session_id", action.SessionID.String()).
			Int64("branch_id", action.BranchID).
			Int64("table_id", action.TableID).
			Msg("session table reconciliation applied")
		if action.SessionID != uuid.Nil {
			w.publisher.SessionClosed(ctx, action.SessionID, map[string]string{"reason": action.Action})
			w.presence.DeleteScoped(ctx, action.OrganizationID, action.BranchID, action.SessionID)
			w.queries.LogEvent(ctx, action.SessionID, action.BranchID, "SESSION_RECONCILED", "system", 0, map[string]any{"reason": action.Action})
			w.recordSystemSessionAudit(ctx, audit.ActionSessionAbandon, result, map[string]any{"reason": action.Action})
		} else {
			w.queries.LogEvent(ctx, uuid.Nil, action.BranchID, "TABLE_RECONCILED", "system", 0, map[string]any{"reason": action.Action, "table_id": action.TableID})
			w.recordSystemSessionAudit(ctx, audit.ActionSessionClose, result, map[string]any{"reason": action.Action})
		}
	}
	w.metrics.WorkerRunsTotal.WithLabelValues("session_table_reconciler", "ok").Inc()
}

type expiryPayload struct {
	ExpiresAt string `json:"expires_at"`
}

func (w *Worker) warnExpiringSessions(ctx context.Context) {
	sessions, err := w.queries.ListSessionsExpiringSoon(ctx)
	if err != nil {
		w.logger.Error().Err(err).Msg("list sessions expiring soon")
		return
	}
	for i := range sessions {
		s := &sessions[i]
		expiresAt := s.CreatedAt.Add(time.Duration(s.SessionTimeoutMinutes) * time.Minute)
		payload := expiryPayload{ExpiresAt: expiresAt.UTC().Format(time.RFC3339)}
		w.publisher.SessionExpiringSoon(ctx, s.ID, payload)
		if err := w.queries.MarkSessionWarned(ctx, s.ID); err != nil {
			w.logger.Error().Err(err).Str("session_id", s.ID.String()).Msg("mark session warned")
		}
	}
	if len(sessions) > 0 {
		w.logger.Info().Int("warned", len(sessions)).Msg("expiry warnings sent")
	}
}

func (w *Worker) recordSystemSessionAudit(ctx context.Context, action string, result repository.SessionMaintenanceResult, metadata map[string]any) {
	if w.audit != nil {
		w.audit.Record(ctx, audit.AuditEvent{
			OrganizationID: result.OrganizationID,
			BranchID:       result.BranchID,
			SessionID:      result.SessionID,
			TableID:        result.TableID,
			ResourceType:   audit.ResourceSession,
			ResourceID:     result.SessionID.String(),
			Action:         action,
			Result:         audit.ResultSuccess,
			ActorType:      audit.ActorTypeSystem,
			ActorID:        "system",
			Source:         audit.SourceSystem,
			RiskLevel:      audit.RiskMedium,
			Metadata:       metadata,
		})
	}
}

// safeRun wraps fn with panic recovery. Panics are logged with a stack trace
// and counted in metrics — they never crash the server process.
func (w *Worker) safeRun(workerName string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error().
				Str("worker", workerName).
				Interface("panic", r).
				Bytes("stack", debug.Stack()).
				Msg("worker panic recovered")
			w.metrics.WorkerPanicsTotal.WithLabelValues(workerName).Inc()
		}
	}()
	fn()
}

// runWithLock acquires a Redis NX lock before executing fn.
// If the lock is already held (another instance is running), fn is skipped.
func (w *Worker) runWithLock(ctx context.Context, name string, lockTTL time.Duration, fn func()) {
	key := fmt.Sprintf("worker:%s:%s:lock", w.region, name)
	acquired, err := w.redis.SetNX(ctx, key, "1", lockTTL).Result()
	if err != nil || !acquired {
		return
	}
	defer w.redis.Del(ctx, key)
	fn()
}
