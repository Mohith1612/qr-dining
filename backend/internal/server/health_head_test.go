package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// probeRoutes are the unauthenticated infrastructure endpoints that external
// monitoring points at. Uptime tooling defaults to HEAD (UptimeRobot does),
// so every one of them has to answer HEAD as well as GET — a 404 there reads
// as an outage even while the service is perfectly healthy.
var probeRoutes = []string{"/health", "/readyz", "/metrics"}

// newProbeServer builds the real router the way main does, with whatever
// Postgres and Redis handles the caller has. Nil/unreachable handles are fine
// for routes that touch no dependency; /readyz needs live ones, so it is
// exercised from the integration test instead.
func newProbeServer(t *testing.T, db *pgxpool.Pool, redisClient *goredis.Client) *Server {
	t.Helper()

	logger := zerolog.Nop()
	metrics := observability.NewMetrics()
	pubsub := redisPkg.NewPubSub(redisClient, logger, metrics)
	publisher := events.NewPublisher(pubsub, logger)
	hub := ws.NewHub(pubsub, metrics, logger, nil)
	repos := repository.New(db, logger)
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode:          gin.TestMode,
			RateLimitRPM:     60,
			AuthRateLimitRPM: 10,
		},
		Auth: config.AuthConfig{
			GuestTokenSecret: "health-head-test-secret",
			GuestTokenTTL:    time.Hour,
		},
		Payment: config.PaymentConfig{
			WebhookSecrets:            map[string]string{},
			WebhookTimestampTolerance: time.Minute,
		},
		Presence: config.PresenceConfig{HostAbsenceGrace: time.Minute},
	}

	return New(cfg, db, redisClient, hub, metrics, logger, repos, publisher)
}

func deadRedis(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestProbeRoutesRegisterHEAD guards the registration itself. Gin does not
// derive HEAD from GET, so a GET-only registration 404s the probe.
func TestProbeRoutesRegisterHEAD(t *testing.T) {
	srv := newProbeServer(t, nil, deadRedis(t))

	byPath := make(map[string][]string)
	for _, route := range srv.router.Routes() {
		byPath[route.Path] = append(byPath[route.Path], route.Method)
	}

	for _, path := range probeRoutes {
		methods := byPath[path]
		sort.Strings(methods)
		if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodHead {
			t.Errorf("%s registered for %v, want [GET HEAD]", path, methods)
		}
	}
}

// TestHeadHealthReturns200WithNoBody runs over a real net/http server rather
// than httptest.NewRecorder: the recorder happily records a body for a HEAD
// request, so it cannot tell a correct HEAD response from a broken one. Only
// a real server applies the "eat the body on HEAD" rule.
func TestHeadHealthReturns200WithNoBody(t *testing.T) {
	srv := newProbeServer(t, nil, deadRedis(t))
	ts := httptest.NewServer(srv.router)
	t.Cleanup(ts.Close)

	assertHeadMatchesGet(t, ts.URL+"/health", http.StatusOK)
}

// assertHeadMatchesGet checks the contract HEAD owes a monitor: the status and
// headers a GET would produce, and an empty body.
func assertHeadMatchesGet(t *testing.T, url string, wantStatus int) {
	t.Helper()

	getReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	getBody, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read GET body: %v", err)
	}
	_ = getResp.Body.Close()

	if getResp.StatusCode != wantStatus {
		t.Fatalf("GET %s = %d, want %d", url, getResp.StatusCode, wantStatus)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, url, nil)
	if err != nil {
		t.Fatalf("build HEAD request: %v", err)
	}
	headResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HEAD %s: %v", url, err)
	}
	headBody, err := io.ReadAll(headResp.Body)
	if err != nil {
		t.Fatalf("read HEAD body: %v", err)
	}
	_ = headResp.Body.Close()

	if headResp.StatusCode != wantStatus {
		t.Errorf("HEAD %s = %d, want %d", url, headResp.StatusCode, wantStatus)
	}
	if len(headBody) != 0 {
		t.Errorf("HEAD %s returned a %d-byte body, want none: %q", url, len(headBody), headBody)
	}

	// A HEAD response must carry the headers GET would have sent, so a monitor
	// reading Content-Length or Content-Type sees the same thing either way.
	for _, header := range []string{"Content-Type", "Content-Length"} {
		if got, want := headResp.Header.Get(header), getResp.Header.Get(header); got != want {
			t.Errorf("HEAD %s %s = %q, want %q (as on GET)", url, header, got, want)
		}
	}
	if int(headResp.ContentLength) != len(getBody) {
		t.Errorf("HEAD %s advertised Content-Length %d, GET body was %d bytes",
			url, headResp.ContentLength, len(getBody))
	}
}
