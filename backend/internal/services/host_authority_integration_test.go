//go:build integration

package services_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func TestPlaceOrder_PresentParticipantBecomesHostAfterHostGrace(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	participantSvc := services.NewParticipantService(repos, pub, presence)
	orderSvc := services.NewOrderService(repos, pub, testMetrics(), services.NewPromoService(repos))
	orderSvc.SetHostAuthority(sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	guest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, created.Participant.ID); err != nil {
		t.Fatalf("host UpdatePresence: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, guest.ID); err != nil {
		t.Fatalf("guest UpdatePresence: %v", err)
	}
	setParticipantLastSeen(t, pool, redisClient, f.OrganizationID, f.BranchID, created.Session.ID, created.Participant.ID, time.Now().Add(-4*time.Minute))

	placed, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: guest.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder after host grace: %v", err)
	}
	if !placed.Order.PlacedByParticipantID.Valid || placed.Order.PlacedByParticipantID.Int64 != guest.ID {
		t.Fatalf("order placed_by: got %+v, want %d", placed.Order.PlacedByParticipantID, guest.ID)
	}

	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != guest.ID {
		t.Fatalf("host participant: got %+v, want %d", persisted.HostParticipantID, guest.ID)
	}
	participants, err := repos.ListParticipantsBySession(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("ListParticipantsBySession: %v", err)
	}
	hostCount := 0
	for _, participant := range participants {
		if participant.IsHost {
			hostCount++
		}
	}
	if hostCount != 1 {
		t.Fatalf("host participant count: got %d, want 1", hostCount)
	}
}

func TestPlaceOrder_OtherHeartbeatsDoNotKeepStaleHostPresent(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	orderSvc := services.NewOrderService(repos, pub, testMetrics(), services.NewPromoService(repos))
	orderSvc.SetHostAuthority(sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	guest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if err := presence.Heartbeat(ctx, created.Session.ID, created.Participant.ID); err != nil {
		t.Fatalf("host Heartbeat: %v", err)
	}
	if err := presence.Heartbeat(ctx, created.Session.ID, guest.ID); err != nil {
		t.Fatalf("guest Heartbeat: %v", err)
	}
	setLegacyParticipantLastSeen(t, pool, redisClient, created.Session.ID, created.Participant.ID, time.Now().Add(-4*time.Minute))
	// A later guest heartbeat refreshes the hash TTL. It must not refresh the
	// host's field-level timestamp or keep the host logically present.
	if err := presence.Heartbeat(ctx, created.Session.ID, guest.ID); err != nil {
		t.Fatalf("continued guest Heartbeat: %v", err)
	}

	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: guest.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("PlaceOrder with stale host field: %v", err)
	}

	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != guest.ID {
		t.Fatalf("host participant: got %+v, want %d", persisted.HostParticipantID, guest.ID)
	}
}

func TestPlaceOrder_HostRetainsAuthorityWithinGrace(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	participantSvc := services.NewParticipantService(repos, pub, presence)
	orderSvc := services.NewOrderService(repos, pub, testMetrics(), services.NewPromoService(repos))
	orderSvc.SetHostAuthority(sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	guest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, created.Participant.ID); err != nil {
		t.Fatalf("host UpdatePresence: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, guest.ID); err != nil {
		t.Fatalf("guest UpdatePresence: %v", err)
	}
	setParticipantLastSeen(t, pool, redisClient, f.OrganizationID, f.BranchID, created.Session.ID, created.Participant.ID, time.Now().Add(-2*time.Minute))

	_, err = orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: guest.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrNotSessionHost) {
		t.Fatalf("guest PlaceOrder within grace: got %v, want ErrNotSessionHost", err)
	}

	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != created.Participant.ID {
		t.Fatalf("host participant: got %+v, want %d", persisted.HostParticipantID, created.Participant.ID)
	}
	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: created.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("host PlaceOrder within grace: %v", err)
	}
}

func TestPlaceOrder_MissingRedisPresenceUsesDurableFiveMinuteFloor(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	participantSvc := services.NewParticipantService(repos, pub, presence)
	orderSvc := services.NewOrderService(repos, pub, testMetrics(), services.NewPromoService(repos))
	orderSvc.SetHostAuthority(sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	guest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, guest.ID); err != nil {
		t.Fatalf("guest UpdatePresence: %v", err)
	}
	setParticipantDBLastSeen(t, pool, created.Participant.ID, time.Now().Add(-4*time.Minute))

	request := services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: guest.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	}
	if _, err := orderSvc.PlaceOrder(ctx, request); !errors.Is(err, domain.ErrNotSessionHost) {
		t.Fatalf("guest PlaceOrder at four-minute durable age: got %v, want ErrNotSessionHost", err)
	}

	setParticipantDBLastSeen(t, pool, created.Participant.ID, time.Now().Add(-6*time.Minute))
	request.IdempotencyKey = uuid.NewString()
	if _, err := orderSvc.PlaceOrder(ctx, request); err != nil {
		t.Fatalf("guest PlaceOrder past durable floor: %v", err)
	}
}

func TestPlaceOrder_AbsentParticipantCannotBecomeHost(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	participantSvc := services.NewParticipantService(repos, pub, presence)
	orderSvc := services.NewOrderService(repos, pub, testMetrics(), services.NewPromoService(repos))
	orderSvc.SetHostAuthority(sessionSvc)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "orders", "order_items", "carts", "cart_items", "idempotency_keys")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	absentGuest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if err := participantSvc.UpdatePresence(ctx, created.Session.ID, created.Participant.ID); err != nil {
		t.Fatalf("host UpdatePresence: %v", err)
	}
	setParticipantLastSeen(t, pool, redisClient, f.OrganizationID, f.BranchID, created.Session.ID, created.Participant.ID, time.Now().Add(-4*time.Minute))

	_, err = orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             created.Session.ID,
		BranchID:              f.BranchID,
		PlacedByParticipantID: absentGuest.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.MenuItemID, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrNotSessionHost) {
		t.Fatalf("absent guest PlaceOrder: got %v, want ErrNotSessionHost", err)
	}

	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != created.Participant.ID {
		t.Fatalf("host participant: got %+v, want %d", persisted.HostParticipantID, created.Participant.ID)
	}
}

func TestTransferHost_RejectsNonPresentTarget(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	absentTarget, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}

	err = sessionSvc.TransferHost(ctx, created.Session.ID, created.Participant.ID, absentTarget.ID)
	if !errors.Is(err, domain.ErrParticipantUnauthorized) {
		t.Fatalf("TransferHost to absent target: got %v, want ErrParticipantUnauthorized", err)
	}
	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != created.Participant.ID {
		t.Fatalf("host participant: got %+v, want %d", persisted.HostParticipantID, created.Participant.ID)
	}
}

func TestGetSnapshot_DoesNotHealHostToAbsentParticipant(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	absentGuest, err := sessionSvc.JoinSession(ctx, created.Session.ID, "Bob", "fp-bob", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE session_participants SET revoked_at = NOW(), revoked_reason = 'left' WHERE id = $1`, created.Participant.ID); err != nil {
		t.Fatalf("revoke host: %v", err)
	}

	snapshot, err := sessionSvc.GetSnapshot(ctx, created.Session.ID, 0)
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if !snapshot.Session.HostParticipantID.Valid || snapshot.Session.HostParticipantID.Int64 != created.Participant.ID {
		t.Fatalf("snapshot host: got %+v, want unchanged host %d", snapshot.Session.HostParticipantID, created.Participant.ID)
	}
	for _, participant := range snapshot.Participants {
		if participant.ID == absentGuest.ID && participant.IsHost {
			t.Fatalf("absent participant %d was promoted by snapshot healing", absentGuest.ID)
		}
	}

	persisted, err := repos.GetSessionByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if !persisted.HostParticipantID.Valid || persisted.HostParticipantID.Int64 != created.Participant.ID {
		t.Fatalf("persisted host: got %+v, want unchanged host %d", persisted.HostParticipantID, created.Participant.ID)
	}
}

func TestUpdatePresence_UnscopedFallbackEmitsWarning(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	redisClient := openServiceTestRedis(t)
	presence := redisPkg.NewPresence(redisClient)
	pub := events.NewNoopPublisher()
	sessionSvc := services.NewSessionService(repos, pub, testMetrics(), presence)
	participantSvc := services.NewParticipantService(repos, pub, presence)
	var logs bytes.Buffer
	participantSvc.SetLogger(zerolog.New(&logs))
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants")
	})

	ctx := context.Background()
	created, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	unknownSessionID := uuid.New()
	if err := participantSvc.UpdatePresence(ctx, unknownSessionID, created.Participant.ID); err != nil {
		t.Fatalf("UpdatePresence fallback: %v", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "presence heartbeat using legacy unscoped fallback") {
		t.Fatalf("fallback warning missing from log: %q", logOutput)
	}
	if !strings.Contains(logOutput, unknownSessionID.String()) {
		t.Fatalf("fallback warning missing session id: %q", logOutput)
	}
}

func openServiceTestRedis(t *testing.T) *goredis.Client {
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

func setParticipantLastSeen(t *testing.T, pool *pgxpool.Pool, redisClient *goredis.Client, organizationID, branchID int64, sessionID uuid.UUID, participantID int64, lastSeen time.Time) {
	t.Helper()
	ctx := context.Background()
	key := fmt.Sprintf("org:%d:branch:%d:session:%s:presence", organizationID, branchID, sessionID)
	if err := redisClient.HSet(ctx, key, strconv.FormatInt(participantID, 10), lastSeen.UTC().Format(time.RFC3339)).Err(); err != nil {
		t.Fatalf("set Redis last seen: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE session_participants SET last_seen_at = $1 WHERE id = $2`, lastSeen, participantID); err != nil {
		t.Fatalf("set DB last seen: %v", err)
	}
}

func setLegacyParticipantLastSeen(t *testing.T, pool *pgxpool.Pool, redisClient *goredis.Client, sessionID uuid.UUID, participantID int64, lastSeen time.Time) {
	t.Helper()
	ctx := context.Background()
	key := "presence:" + sessionID.String()
	if err := redisClient.HSet(ctx, key, strconv.FormatInt(participantID, 10), lastSeen.UTC().Format(time.RFC3339)).Err(); err != nil {
		t.Fatalf("set legacy Redis last seen: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE session_participants SET last_seen_at = $1 WHERE id = $2`, lastSeen, participantID); err != nil {
		t.Fatalf("set DB last seen: %v", err)
	}
}

func setParticipantDBLastSeen(t *testing.T, pool *pgxpool.Pool, participantID int64, lastSeen time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE session_participants SET last_seen_at = $1 WHERE id = $2`, lastSeen, participantID); err != nil {
		t.Fatalf("set DB last seen: %v", err)
	}
}
