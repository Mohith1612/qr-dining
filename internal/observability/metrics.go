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
	WSConnectionsActive  prometheus.Gauge
	WSMessagesSentTotal  *prometheus.CounterVec
	WSClientEvictions    prometheus.Counter

	// Database
	DBQueryDuration *prometheus.HistogramVec
	DBErrorsTotal   *prometheus.CounterVec

	// Redis
	RedisOpsTotal *prometheus.CounterVec

	// Business
	ActiveSessionsTotal prometheus.Gauge
	OrdersTotal         *prometheus.CounterVec

	// Background workers
	WorkerRunsTotal *prometheus.CounterVec
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

		DBQueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency distribution.",
			Buckets: prometheus.DefBuckets,
		}, []string{"query"}),

		DBErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "db_errors_total",
			Help: "Total database errors by query name.",
		}, []string{"query"}),

		RedisOpsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "redis_ops_total",
			Help: "Total Redis operations by operation type and status.",
		}, []string{"op", "status"}),

		ActiveSessionsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Current number of active dining sessions.",
		}),

		OrdersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orders_total",
			Help: "Total orders placed by resulting status.",
		}, []string{"status"}),

		WorkerRunsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "background_worker_runs_total",
			Help: "Total background worker executions by worker name and status.",
		}, []string{"worker", "status"}),
	}

	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.WSConnectionsActive,
		m.WSMessagesSentTotal,
		m.WSClientEvictions,
		m.DBQueryDuration,
		m.DBErrorsTotal,
		m.RedisOpsTotal,
		m.ActiveSessionsTotal,
		m.OrdersTotal,
		m.WorkerRunsTotal,
	)

	return m
}
