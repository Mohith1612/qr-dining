//go:build integration

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/testutil"
	goredis "github.com/redis/go-redis/v9"
)

// TestHeadReadyzReturns200WithNoBody covers the endpoint the false outage was
// actually reported against. /readyz only reaches 200 when Postgres and Redis
// are both live, so it needs the integration harness — but the contract under
// test is the same one /health has: HEAD answers, and answers without a body.
func TestHeadReadyzReturns200WithNoBody(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	redisClient := openTestRedisForProbes(t)

	srv := newProbeServer(t, pool, redisClient)
	ts := httptest.NewServer(srv.router)
	t.Cleanup(ts.Close)

	assertHeadMatchesGet(t, ts.URL+"/readyz", http.StatusOK)
}

func openTestRedisForProbes(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse TEST_REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("ping test Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
