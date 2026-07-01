package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metric definitions.
// Uses a custom registry (not the default global) to allow clean testing.
type Metrics struct {
	Registry *prometheus.Registry

	// HTTP
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// WebSocket
	WSConnectionsActive prometheus.Gauge
	WSMessagesSentTotal *prometheus.CounterVec
	WSClientEvictions   prometheus.Counter
	WSReconnectsTotal   prometheus.Counter

	// WebSocket inbound abuse hardening
	WSInboundMessagesTotal prometheus.Counter
	WSInboundDroppedTotal  prometheus.Counter
	WSMalformedEventsTotal prometheus.Counter
	WSAbusiveClosesTotal   prometheus.Counter

	// Database
	DBQueryDuration     *prometheus.HistogramVec
	DBErrorsTotal       *prometheus.CounterVec
	DBPoolTotalConns    prometheus.Gauge
	DBPoolIdleConns     prometheus.Gauge
	DBPoolAcquiredConns prometheus.Gauge
	DBPoolAcquireCount  prometheus.Counter

	// Redis
	RedisOpsTotal        *prometheus.CounterVec
	CacheHitsTotal       prometheus.Counter
	CacheMissesTotal     prometheus.Counter
	RedisPubSubConnected prometheus.Gauge
	RedisPubSubErrors    *prometheus.CounterVec
	RedisReconnectsTotal prometheus.Counter
	EventPublishDuration *prometheus.HistogramVec

	// Business
	ActiveSessionsTotal      prometheus.Gauge
	OrdersTotal              *prometheus.CounterVec
	OrderLifecycleDuration   *prometheus.HistogramVec
	SessionDuration          prometheus.Histogram
	IdempotencyReplaysTotal  *prometheus.CounterVec
	LegacyIdentityUsageTotal *prometheus.CounterVec

	// Background workers
	WorkerRunsTotal   *prometheus.CounterVec
	WorkerPanicsTotal *prometheus.CounterVec

	// Audit
	AuditWriteFailuresTotal *prometheus.CounterVec

	// LegacyAuthzBypassTotal counts policy denials that were allowed through
	// because AUTHZ_CENTRAL_POLICY_ENFORCE was off. A spike here right before
	// the cutover means the strict flip will break legitimate traffic.
	LegacyAuthzBypassTotal *prometheus.CounterVec

	// Rollout instrumentation (Phase E). Gate the staged strict-flag rollout.
	//
	// AuthzDeniedTotal counts real denials after AUTHZ_CENTRAL_POLICY_ENFORCE is on.
	// PolicyShadowMismatchTotal counts requests the policy WOULD deny while enforce
	// is off (the pre-flip would-break signal, labelled by route for pinpointing).
	// TenantResolutionFailuresTotal counts org/restaurant resolution failures.
	// GuestTokenValidationFailedTotal / WSTicketConsumeFailedTotal count guest and
	// websocket-ticket auth failures by bounded reason.
	// PaymentPendingEscalationsTotal counts stalled payment_pending escalations by level.
	AuthzDeniedTotal                *prometheus.CounterVec
	PolicyShadowMismatchTotal       *prometheus.CounterVec
	TenantResolutionFailuresTotal   *prometheus.CounterVec
	GuestTokenValidationFailedTotal *prometheus.CounterVec
	WSTicketConsumeFailedTotal      *prometheus.CounterVec
	PaymentPendingEscalationsTotal  *prometheus.CounterVec

	// EntitlementEvaluationsTotal counts organization entitlement capability
	// evaluations by capability and result. Shadow-only: the platform governance
	// foundation resolves entitlements without enforcing them on operational
	// paths, so a spike in "deny" here is a pre-enforcement would-break signal.
	EntitlementEvaluationsTotal *prometheus.CounterVec

	// RateLimiterUnavailableTotal counts requests denied because the rate
	// limiter backend was unreachable on a fail-closed surface (staff auth,
	// payment, webhook, ws-ticket).
	RateLimiterUnavailableTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		Registry: reg,

		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, path pattern, and status code bucket.",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency distribution.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		WSConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ws_connections_active",
			Help: "Number of currently active WebSocket connections.",
		}),

		WSMessagesSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ws_messages_sent_total",
			Help: "Total WebSocket messages sent by event type.",
		}, []string{"event_type"}),

		WSClientEvictions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_client_evictions_total",
			Help: "Total WebSocket clients evicted due to slow consumer (full send buffer).",
		}),

		WSReconnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_reconnects_total",
			Help: "Total WebSocket reconnect events detected (participant already had an active client).",
		}),

		WSInboundMessagesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_inbound_messages_total",
			Help: "Total inbound WebSocket frames read from clients (before rate-limit/validation).",
		}),

		WSInboundDroppedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_inbound_dropped_total",
			Help: "Total inbound WebSocket frames dropped because the per-connection inbound rate limit was exceeded.",
		}),

		WSMalformedEventsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_malformed_events_total",
			Help: "Total inbound WebSocket frames that failed envelope parsing.",
		}),

		WSAbusiveClosesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ws_abusive_closes_total",
			Help: "Total WebSocket connections force-closed after exceeding the inbound abuse strike budget.",
		}),

		DBQueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency distribution.",
			Buckets: prometheus.DefBuckets,
		}, []string{"query"}),

		DBErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "db_errors_total",
			Help: "Total database errors by query name.",
		}, []string{"query"}),

		DBPoolTotalConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_total_conns",
			Help: "Total connections in the PostgreSQL connection pool.",
		}),

		DBPoolIdleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_idle_conns",
			Help: "Idle connections in the PostgreSQL connection pool.",
		}),

		DBPoolAcquiredConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_acquired_conns",
			Help: "Currently acquired (in-use) connections in the PostgreSQL pool.",
		}),

		DBPoolAcquireCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "db_pool_acquire_count_total",
			Help: "Total number of connection acquire calls against the PostgreSQL pool.",
		}),

		RedisOpsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "redis_ops_total",
			Help: "Total Redis operations by operation type and status.",
		}, []string{"op", "status"}),

		CacheHitsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total Redis cache hits (key found).",
		}),

		CacheMissesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total Redis cache misses (key not found, DB fallback required).",
		}),

		RedisPubSubConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_pubsub_connected",
			Help: "1 if the Redis pub/sub subscriber is active, 0 if disconnected.",
		}),

		RedisPubSubErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "redis_pubsub_errors_total",
			Help: "Total Redis pub/sub errors by operation (subscribe, receive, publish).",
		}, []string{"operation"}),

		RedisReconnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "redis_pubsub_reconnects_total",
			Help: "Total times the Redis pub/sub subscriber has been restarted after an error.",
		}),

		EventPublishDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "event_publish_duration_seconds",
			Help:    "Server-side time to persist (append session event) and publish a realtime event to Redis, by event type and outcome.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
		}, []string{"event", "outcome"}),

		ActiveSessionsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Current number of active dining sessions.",
		}),

		OrdersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orders_total",
			Help: "Total orders placed by resulting status.",
		}, []string{"status"}),

		OrderLifecycleDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "order_lifecycle_duration_seconds",
			Help:    "Time elapsed between order status transitions.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600},
		}, []string{"from_status", "to_status"}),

		SessionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "session_duration_seconds",
			Help:    "Time from session creation to close.",
			Buckets: []float64{60, 300, 600, 1200, 1800, 3600, 7200},
		}),

		IdempotencyReplaysTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "idempotency_replays_total",
			Help: "Total idempotency replay responses by entity type.",
		}, []string{"entity"}),

		LegacyIdentityUsageTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "legacy_identity_usage_total",
			Help: "Total uses of legacy client-supplied identity mechanisms by mechanism and endpoint class.",
		}, []string{"mechanism", "endpoint_class"}),

		WorkerRunsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "background_worker_runs_total",
			Help: "Total background worker executions by worker name and status.",
		}, []string{"worker", "status"}),

		WorkerPanicsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "background_worker_panics_total",
			Help: "Total panics recovered in background workers by worker name.",
		}, []string{"worker"}),

		AuditWriteFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "audit_write_failures_total",
			Help: "Total audit_log write failures by action and error class.",
		}, []string{"action", "error_class"}),

		LegacyAuthzBypassTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "legacy_authz_bypass_total",
			Help: "Total policy denials allowed through because AUTHZ_CENTRAL_POLICY_ENFORCE was off.",
		}, []string{"action", "actor_role"}),

		RateLimiterUnavailableTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rate_limiter_unavailable_total",
			Help: "Total requests denied because the rate-limit backend was unreachable on a fail-closed surface.",
		}, []string{"surface"}),

		AuthzDeniedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "authz_denied_total",
			Help: "Total authorization denials enforced (AUTHZ_CENTRAL_POLICY_ENFORCE on) by reason.",
		}, []string{"reason"}),

		PolicyShadowMismatchTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "policy_shadow_mismatch_total",
			Help: "Requests the central policy would deny while enforcement is off (pre-flip would-break signal) by route and reason.",
		}, []string{"route", "reason"}),

		TenantResolutionFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tenant_resolution_failures_total",
			Help: "Total tenant/organization resolution failures in tenant middleware by stage.",
		}, []string{"stage"}),

		GuestTokenValidationFailedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "guest_token_validation_failed_total",
			Help: "Total guest token validation failures by bounded reason.",
		}, []string{"reason"}),

		WSTicketConsumeFailedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ws_ticket_consume_failed_total",
			Help: "Total websocket ticket consume failures by bounded reason.",
		}, []string{"reason"}),

		PaymentPendingEscalationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "payment_pending_escalations_total",
			Help: "Total stalled payment_pending escalations emitted by the escalation worker, by level.",
		}, []string{"level"}),

		EntitlementEvaluationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "entitlement_evaluations_total",
			Help: "Total organization entitlement capability evaluations by capability and result (allow|deny). Shadow-only; not enforced on operational paths.",
		}, []string{"capability", "result"}),
	}

	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.WSConnectionsActive,
		m.WSMessagesSentTotal,
		m.WSClientEvictions,
		m.WSReconnectsTotal,
		m.WSInboundMessagesTotal,
		m.WSInboundDroppedTotal,
		m.WSMalformedEventsTotal,
		m.WSAbusiveClosesTotal,
		m.DBQueryDuration,
		m.DBErrorsTotal,
		m.DBPoolTotalConns,
		m.DBPoolIdleConns,
		m.DBPoolAcquiredConns,
		m.DBPoolAcquireCount,
		m.RedisOpsTotal,
		m.CacheHitsTotal,
		m.CacheMissesTotal,
		m.RedisPubSubConnected,
		m.RedisPubSubErrors,
		m.RedisReconnectsTotal,
		m.EventPublishDuration,
		m.ActiveSessionsTotal,
		m.OrdersTotal,
		m.OrderLifecycleDuration,
		m.SessionDuration,
		m.IdempotencyReplaysTotal,
		m.LegacyIdentityUsageTotal,
		m.WorkerRunsTotal,
		m.WorkerPanicsTotal,
		m.AuditWriteFailuresTotal,
		m.LegacyAuthzBypassTotal,
		m.RateLimiterUnavailableTotal,
		m.AuthzDeniedTotal,
		m.PolicyShadowMismatchTotal,
		m.TenantResolutionFailuresTotal,
		m.GuestTokenValidationFailedTotal,
		m.WSTicketConsumeFailedTotal,
		m.PaymentPendingEscalationsTotal,
		m.EntitlementEvaluationsTotal,
	)

	return m
}
