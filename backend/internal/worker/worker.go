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
	LastActivityAt        time.Time
	SessionTimeoutMinutes int16
}

// ReactivationCandidate is a session that may need to move from active into
// awaiting_reactivation.
type ReactivationCandidate struct {
	ID             uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
}

// AwaitingReactivationExpired is a session that has overstayed the
// reactivation window and is eligible for abandonment.
type AwaitingReactivationExpired struct {
	ID             uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
}

// StalledPaymentPending is a session stuck in payment_pending past the
// escalation threshold (oldest non-terminal payment's initiated_at).
type StalledPaymentPending struct {
	SessionID      uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
	PaymentID      int64
	InitiatedAt    time.Time
}

// BillingReconciliationDiscrepancy is the public one-pass result returned to
// operational callers and integration tests.
type BillingReconciliationDiscrepancy = repository.BillingReconciliationDiscrepancy

// Querier abstracts the DB queries the worker needs.
// Implemented by *sqlc.Queries after code generation; interface allows compilation before that.
type Querier interface {
	ListExpiredSessions(ctx context.Context) ([]ExpiredSession, error)
	AbandonStaleSession(ctx context.Context, id uuid.UUID, tableID int64) (repository.SessionMaintenanceResult, bool, error)
	ListSessionsExpiringSoon(ctx context.Context) ([]ExpiringSoonSession, error)
	MarkSessionWarned(ctx context.Context, id uuid.UUID) error
	ReconcileSessionTables(ctx context.Context) ([]repository.SessionTableReconciliation, error)
	LogEvent(ctx context.Context, sessionID uuid.UUID, branchID int64, eventType, actorType string, actorID int64, payload any)

	ListReactivationCandidates(ctx context.Context, createdBefore, idleBefore time.Time) ([]ReactivationCandidate, error)
	TransitionToAwaitingReactivation(ctx context.Context, id uuid.UUID) error
	ListAwaitingReactivationExpired(ctx context.Context, olderThan time.Time) ([]AwaitingReactivationExpired, error)
	HasNonTerminalPayment(ctx context.Context, sessionID uuid.UUID) (bool, error)
	ListPaymentPendingStalled(ctx context.Context, olderThan time.Time) ([]StalledPaymentPending, error)
	ListBillingReconciliationDiscrepancies(ctx context.Context, windowStart, windowEnd time.Time) ([]BillingReconciliationDiscrepancy, error)
	DeleteExpiredIdempotencyKeys(ctx context.Context, expiredBefore time.Time, limit int32) (int64, error)
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
	// sessionIdleGrace is the durable no-presence interval required before an
	// established active session may enter awaiting_reactivation.
	sessionIdleGrace time.Duration
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
		db:               db,
		queries:          queries,
		redis:            redis,
		publisher:        publisher,
		presence:         presence,
		metrics:          metrics,
		audit:            auditWriter,
		region:           region,
		logger:           logger.With().Str("component", "worker").Logger(),
		sessionIdleGrace: 5 * time.Minute,
	}
}

// ReconcileBilling performs one read-only reconciliation pass over settled
// sessions in [windowStart, windowEnd). It never corrects, cancels, settles, or
// otherwise mutates payments, bills, orders, promos, loyalty, or sessions.
// Audit rows and metrics are observations only; money remains operator-owned.
func (w *Worker) ReconcileBilling(ctx context.Context, windowStart, windowEnd time.Time) ([]BillingReconciliationDiscrepancy, error) {
	findings, err := w.queries.ListBillingReconciliationDiscrepancies(ctx, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{
		"snapshot_vs_orders":    0,
		"collected_vs_snapshot": 0,
	}
	for _, finding := range findings {
		counts[finding.Comparison]++
		if w.audit != nil && !finding.AlreadyAudited {
			w.audit.RecordRequired(ctx, audit.AuditEvent{
				OrganizationID: finding.OrganizationID,
				BranchID:       finding.BranchID,
				SessionID:      finding.SessionID,
				TableID:        finding.TableID,
				ResourceType:   audit.ResourceSession,
				ResourceID:     finding.SessionID.String(),
				Action:         audit.ActionBillingReconciliationDiscrepancy,
				Result:         audit.ResultFailure,
				ActorType:      audit.ActorTypeSystem,
				ActorID:        "system",
				Source:         audit.SourceSystem,
				RiskLevel:      audit.RiskCritical,
				Metadata: map[string]any{
					"bill_snapshot_id": finding.BillSnapshotID,
					"comparison":       finding.Comparison,
					"expected_amount":  finding.ExpectedAmount,
					"actual_amount":    finding.ActualAmount,
					"difference":       finding.Difference,
					"currency":         finding.Currency,
				},
			})
		}
	}
	if w.metrics != nil && w.metrics.BillingReconciliationDiscrepancies != nil {
		for comparison, count := range counts {
			w.metrics.BillingReconciliationDiscrepancies.WithLabelValues(comparison).Set(float64(count))
		}
	}
	return findings, nil
}

// SetSessionIdleGrace overrides the five-minute default. It must be called at
// startup before RunReactivationPipeline begins.
func (w *Worker) SetSessionIdleGrace(grace time.Duration) {
	if grace > 0 {
		w.sessionIdleGrace = grace
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
// participants whose last heartbeat has expired. Live presence expires by
// field age; the Redis key TTL is storage cleanup. This worker ensures
// WebSocket clients get a notification.
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
					// Field-age filtering is applied by presence readers; this remains a
					// no-op notification hook until participant-left events are needed.
					w.logger.Debug().Msg("presence expiry worker tick")
				})
			})
		}
	}
}

// RunReactivationPipeline drives the awaiting_reactivation lifecycle:
//   - Active sessions past the creation grace whose newest durable participant
//     heartbeat is older than sessionIdleGrace and whose live Redis presence is
//     empty are moved to awaiting_reactivation. The table stays occupied.
//   - awaiting_reactivation sessions whose `awaiting_reactivation_at` is older
//     than `reactivationWindow` are abandoned, unless a non-terminal payment
//     exists (TIM-1).
//
// The pipeline is conservative: it never bypasses payment_pending sessions
// and it relies on the same advisory locking the stale-session worker uses
// for concurrency safety with webhooks.
func (w *Worker) RunReactivationPipeline(ctx context.Context, interval, presenceGrace, reactivationWindow time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "reactivation_pipeline", 4*time.Minute, func() {
				w.safeRun("reactivation_pipeline", func() {
					tickCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
					defer cancel()
					w.runReactivationPipeline(tickCtx, presenceGrace, reactivationWindow)
				})
			})
		}
	}
}

// RunPaymentPendingEscalation surfaces sessions stuck in payment_pending past
// the warn/critical thresholds. It is ALERT-ONLY: it never mutates payment or
// session state (auto-settle/auto-cancel would violate settlement invariants).
// Recovery stays operator-driven: staff settle via PATCH /payments/:id/settle
// or cancel via PATCH /payments/:id/cancel, and can force-close the session
// with POST /sessions/:id/force-close.
func (w *Worker) RunPaymentPendingEscalation(ctx context.Context, interval, warnAfter, criticalAfter time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "payment_pending_escalation", 50*time.Second, func() {
				w.safeRun("payment_pending_escalation", func() {
					tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					w.escalatePaymentPending(tickCtx, warnAfter, criticalAfter)
				})
			})
		}
	}
}

const (
	billingReconciliationLookback    = 24 * time.Hour
	billingReconciliationQuietPeriod = time.Minute
)

// RunBillingReconciliation observes recently settled sessions for A1 billing
// discrepancies. It is strictly read-only with respect to money: it must never
// correct, cancel, settle, refund, or adjust financial/session state. Its only
// writes are reporting signals (Prometheus and immutable audit records).
func (w *Worker) RunBillingReconciliation(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "billing_reconciliation", 50*time.Second, func() {
				w.safeRun("billing_reconciliation", func() {
					// Keep execution below the lock TTL. runWithLock's bare DEL is
					// unfenced, so this worker must not intentionally outlive its lock.
					tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					windowEnd := time.Now().UTC().Add(-billingReconciliationQuietPeriod)
					findings, err := w.ReconcileBilling(tickCtx, windowEnd.Add(-billingReconciliationLookback), windowEnd)
					if err != nil {
						w.logger.Error().Err(err).Msg("reconcile settled-session billing")
						w.metrics.WorkerRunsTotal.WithLabelValues("billing_reconciliation", "error").Inc()
						return
					}
					if len(findings) > 0 {
						w.logger.Error().Int("discrepancies", len(findings)).Msg("billing reconciliation discrepancies found")
					}
					w.metrics.WorkerRunsTotal.WithLabelValues("billing_reconciliation", "ok").Inc()
				})
			})
		}
	}
}

const (
	// IdempotencyKeyRetention is how long a key is kept AFTER its expires_at
	// has passed. Expiry is enforced on the lookup path, not here, so this
	// window changes no request's outcome — it is purely operational: it keeps
	// the row readable for a support engineer triaging "was I charged twice on
	// Saturday?" across a weekend, while still bounding the table to roughly
	// the 24h live window plus this grace.
	IdempotencyKeyRetention = 7 * 24 * time.Hour

	// idempotencyReapBatchSize bounds a single DELETE so no statement runs long
	// against a backlogged table.
	idempotencyReapBatchSize = 1000

	// The run budget sits strictly below the lock TTL. runWithLock releases with
	// a bare DEL and carries no fencing token, so a run that outlived its lock
	// could delete a lock another instance had since acquired. Same constraint
	// the billing reconciler documents.
	idempotencyReapLockTTL   = 50 * time.Second
	idempotencyReapRunBudget = 30 * time.Second
)

// RunIdempotencyKeyReaper bounds the growth of idempotency_keys, which nothing
// swept before: every reservation made by the order and payment paths stayed in
// the table forever. Uses idx_idempotency_keys_expires_at from migration
// 000021, which was created for exactly this and then went unused.
func (w *Worker) RunIdempotencyKeyReaper(ctx context.Context, interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithLock(ctx, "idempotency_key_reaper", idempotencyReapLockTTL, func() {
				w.safeRun("idempotency_key_reaper", func() {
					tickCtx, cancel := context.WithTimeout(ctx, idempotencyReapRunBudget)
					defer cancel()
					deleted, err := w.ReapIdempotencyKeys(tickCtx, retention)
					if err != nil {
						w.logger.Error().Err(err).Msg("reap expired idempotency keys")
						w.metrics.WorkerRunsTotal.WithLabelValues("idempotency_key_reaper", "error").Inc()
						return
					}
					if deleted > 0 {
						w.logger.Info().Int64("deleted", deleted).Msg("reaped expired idempotency keys")
					}
					w.metrics.WorkerRunsTotal.WithLabelValues("idempotency_key_reaper", "ok").Inc()
				})
			})
		}
	}
}

// ReapIdempotencyKeys deletes, in bounded batches, every idempotency key whose
// expiry passed more than retention ago, and reports how many it removed.
// Exported so operators and integration tests can drive one deterministic pass.
//
// Strictly a janitor. An expired key already stops replaying at the lookup (see
// sql/queries/idempotency.sql), so deleting its row changes no request's
// outcome — which is what makes it safe to run on any schedule at all.
func (w *Worker) ReapIdempotencyKeys(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-retention)
	var total int64
	for {
		if ctx.Err() != nil {
			// Budget spent mid-backlog. Keep what we removed; the next tick resumes.
			return total, nil
		}
		deleted, err := w.queries.DeleteExpiredIdempotencyKeys(ctx, cutoff, idempotencyReapBatchSize)
		if err != nil {
			if ctx.Err() != nil {
				return total, nil
			}
			return total, err
		}
		total += deleted
		if deleted < idempotencyReapBatchSize {
			return total, nil
		}
	}
}

func (w *Worker) escalatePaymentPending(ctx context.Context, warnAfter, criticalAfter time.Duration) {
	now := time.Now().UTC()
	stalled, err := w.queries.ListPaymentPendingStalled(ctx, now.Add(-warnAfter))
	if err != nil {
		w.logger.Error().Err(err).Msg("list stalled payment_pending")
		w.metrics.WorkerRunsTotal.WithLabelValues("payment_pending_escalation", "error").Inc()
		return
	}

	warned, critical := 0, 0
	for _, s := range stalled {
		ageSeconds := int64(now.Sub(s.InitiatedAt).Seconds())
		level := "warn"
		if now.Sub(s.InitiatedAt) >= criticalAfter {
			level = "critical"
		}
		// De-dupe: emit once per session per level. Marker outlives the critical
		// window so a session isn't re-escalated at the same level every tick.
		if !w.markEscalated(ctx, s.SessionID, level, 2*criticalAfter) {
			continue
		}

		w.metrics.PaymentPendingEscalationsTotal.WithLabelValues(level).Inc()
		w.logger.Warn().
			Str("session_id", s.SessionID.String()).
			Int64("payment_id", s.PaymentID).
			Int64("branch_id", s.BranchID).
			Str("level", level).
			Int64("age_seconds", ageSeconds).
			Msg("payment_pending settlement stalled")

		payload := map[string]any{"level": level, "age_seconds": ageSeconds, "payment_id": s.PaymentID}
		w.queries.LogEvent(ctx, s.SessionID, s.BranchID, "PAYMENT_SETTLEMENT_STALLED", "system", 0, payload)
		w.publisher.PaymentSettlementStalled(ctx, s.SessionID, payload)
		if w.audit != nil {
			w.audit.Record(ctx, audit.AuditEvent{
				OrganizationID: s.OrganizationID,
				BranchID:       s.BranchID,
				SessionID:      s.SessionID,
				TableID:        s.TableID,
				ResourceType:   audit.ResourcePayment,
				ResourceID:     fmt.Sprintf("%d", s.PaymentID),
				Action:         audit.ActionPaymentSettlementStalled,
				Result:         audit.ResultSuccess,
				ActorType:      audit.ActorTypeSystem,
				ActorID:        "system",
				Source:         audit.SourceSystem,
				RiskLevel:      audit.RiskMedium,
				Metadata:       payload,
			})
		}

		if level == "critical" {
			critical++
		} else {
			warned++
		}
	}

	if warned+critical > 0 {
		w.logger.Info().Int("warned", warned).Int("critical", critical).Msg("payment pending escalation")
	}
	w.metrics.WorkerRunsTotal.WithLabelValues("payment_pending_escalation", "ok").Inc()
}

// markEscalated records a (session,level) escalation in Redis and reports whether
// this is the first time (true) so the same level fires only once per session.
func (w *Worker) markEscalated(ctx context.Context, sessionID uuid.UUID, level string, ttl time.Duration) bool {
	key := fmt.Sprintf("payment_escalated:%s:%s", level, sessionID.String())
	ok, err := w.redis.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		// Fail open: better to risk a duplicate alert than to suppress a real one.
		return true
	}
	return ok
}

func (w *Worker) runReactivationPipeline(ctx context.Context, presenceGrace, reactivationWindow time.Duration) {
	// Phase 1: active → awaiting_reactivation only after both the creation
	// grace and the durable participant-idle grace, with Redis still empty.
	now := time.Now().UTC()
	candidates, err := w.queries.ListReactivationCandidates(
		ctx,
		now.Add(-presenceGrace),
		now.Add(-w.sessionIdleGrace),
	)
	if err != nil {
		w.logger.Error().Err(err).Msg("list reactivation candidates")
		w.metrics.WorkerRunsTotal.WithLabelValues("reactivation_pipeline", "error").Inc()
		return
	}
	movedToAwaiting := 0
	for _, c := range candidates {
		present, err := w.presence.GetPresentForSession(ctx, c.OrganizationID, c.BranchID, c.ID)
		if err != nil {
			w.logger.Warn().Err(err).Str("session_id", c.ID.String()).Msg("read presence")
			continue
		}
		if len(present) > 0 {
			continue
		}
		// Don't tear into payment_pending — that has its own state machine.
		hasPending, err := w.queries.HasNonTerminalPayment(ctx, c.ID)
		if err == nil && hasPending {
			continue
		}
		if err := w.queries.TransitionToAwaitingReactivation(ctx, c.ID); err != nil {
			w.logger.Warn().Err(err).Str("session_id", c.ID.String()).Msg("transition to awaiting_reactivation")
			continue
		}
		movedToAwaiting++
		w.queries.LogEvent(ctx, c.ID, c.BranchID, "SESSION_AWAITING_REACTIVATION", "system", 0, map[string]any{"reason": "presence_lost"})
		w.publisher.SessionExpiringSoon(ctx, c.ID, map[string]any{"reason": "presence_lost"})
	}

	// Phase 2: awaiting_reactivation → abandoned when window has elapsed and
	// no payment is in flight.
	expired, err := w.queries.ListAwaitingReactivationExpired(ctx, time.Now().UTC().Add(-reactivationWindow))
	if err != nil {
		w.logger.Error().Err(err).Msg("list expired awaiting_reactivation")
		w.metrics.WorkerRunsTotal.WithLabelValues("reactivation_pipeline", "error").Inc()
		return
	}
	abandoned := 0
	for _, e := range expired {
		hasPending, err := w.queries.HasNonTerminalPayment(ctx, e.ID)
		if err != nil {
			w.logger.Warn().Err(err).Str("session_id", e.ID.String()).Msg("payment check")
			continue
		}
		if hasPending {
			// Staff intervention required — surface to ops.
			w.logger.Warn().Str("session_id", e.ID.String()).Msg("awaiting_reactivation expired but payment is in flight; skipping abandon")
			continue
		}
		result, didAbandon, err := w.queries.AbandonStaleSession(ctx, e.ID, e.TableID)
		if err != nil {
			w.logger.Error().Err(err).Str("session_id", e.ID.String()).Msg("abandon awaiting_reactivation")
			continue
		}
		if !didAbandon {
			continue
		}
		w.publisher.SessionClosed(ctx, e.ID, map[string]string{"reason": "reactivation_window_expired"})
		w.presence.DeleteScoped(ctx, result.OrganizationID, result.BranchID, e.ID)
		w.queries.LogEvent(ctx, result.SessionID, result.BranchID, "SESSION_ABANDONED", "system", 0, map[string]any{"reason": "reactivation_window_expired"})
		w.recordSystemSessionAudit(ctx, audit.ActionSessionAbandon, result, map[string]any{"reason": "reactivation_window_expired"})
		abandoned++
	}

	if movedToAwaiting > 0 || abandoned > 0 {
		w.logger.Info().Int("awaiting", movedToAwaiting).Int("abandoned", abandoned).Msg("reactivation pipeline")
	}
	w.metrics.WorkerRunsTotal.WithLabelValues("reactivation_pipeline", "ok").Inc()
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
			if action.Action == "duplicate_abandoned" {
				w.publisher.SessionClosed(ctx, action.SessionID, map[string]string{"reason": action.Action})
				w.presence.DeleteScoped(ctx, action.OrganizationID, action.BranchID, action.SessionID)
			}
			w.queries.LogEvent(ctx, action.SessionID, action.BranchID, "SESSION_RECONCILED", "system", 0, map[string]any{"reason": action.Action})
			if action.Action == "duplicate_abandoned" {
				w.recordSystemSessionAudit(ctx, audit.ActionSessionAbandon, result, map[string]any{"reason": action.Action})
			}
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
		expiresAt := s.LastActivityAt.Add(time.Duration(s.SessionTimeoutMinutes) * time.Minute)
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
