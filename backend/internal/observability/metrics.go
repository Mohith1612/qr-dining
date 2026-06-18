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
		m.ActiveSessionsTotal,
		m.OrdersTotal,
		m.OrderLifecycleDuration,
		m.SessionDuration,
		m.IdempotencyReplaysTotal,
		m.LegacyIdentityUsageTotal,
		m.WorkerRunsTotal,
		m.WorkerPanicsTotal,
	)

	return m
}
