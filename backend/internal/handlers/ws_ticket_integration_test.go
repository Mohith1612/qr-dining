//go:build integration

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/config"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

func TestWSUpgradeWithTicketRejectsCredentialVersionMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testutil.OpenTestDB(t)
	redisClient := openHandlerTestRedis(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO sessions (branch_id, table_id, session_token, status, session_business_date, visit_number, session_number)
		 VALUES ($1, $2, $3, 'active', CURRENT_DATE, 1, $4) RETURNING id`,
		f.BranchID, f.TableID, uuid.NewString(), "ws-ticket-"+uuid.NewString(),
	).Scan(&sessionID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	var participantID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO session_participants (session_id, display_name, credential_version)
		 VALUES ($1, 'Guest', 2) RETURNING id`,
		sessionID,
	).Scan(&participantID); err != nil {
		t.Fatalf("insert participant: %v", err)
	}

	tickets := redisPkg.NewWSTicketStore(redisClient)
	ticket, err := tickets.Issue(ctx, redisPkg.WSTicketClaims{
		SessionID:         sessionID,
		OrganizationID:    f.OrganizationID,
		BranchID:          f.BranchID,
		ParticipantID:     participantID,
		CredentialVersion: 1,
		JTI:               uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("Issue ticket: %v", err)
	}

	handler := NewWSHandler(nil, repos, nil, nil, tickets, config.FeatureFlags{WSTicketAuthRequired: true})
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/ws?ticket="+ticket, nil)

	handler.Upgrade(ginCtx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func openHandlerTestRedis(t *testing.T) *goredis.Client {
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
		t.Fatalf("ping redis: %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}
