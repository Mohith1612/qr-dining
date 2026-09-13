//go:build integration

package services_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// F-08: reactivation was announced as SESSION_CREATED. No client handler could
// act on that — SESSION_REACTIVATED existed only as an audit string — so the
// "table paused" overlay stayed up. These tests pin the event contract.

func eventsContain(events []string, want string) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}

func sessionEventsFor(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT event FROM session_events WHERE session_id = $1 ORDER BY sequence`, sessionID)
	if err != nil {
		t.Fatalf("query session_events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatalf("scan session_events: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate session_events: %v", err)
	}
	return out
}

func sessionEventPayload(t *testing.T, pool *pgxpool.Pool, sessionID uuid.UUID, event string) map[string]any {
	t.Helper()
	var raw []byte
	err := pool.QueryRow(context.Background(),
		`SELECT payload FROM session_events WHERE session_id = $1 AND event = $2 ORDER BY sequence DESC LIMIT 1`,
		sessionID, event).Scan(&raw)
	if err != nil {
		t.Fatalf("read %s payload: %v", event, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s payload: %v", event, err)
	}
	return out
}

func TestReactivate_PublishesSessionReactivated(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	redisClient := openServiceTestRedis(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)

	metrics := observability.NewMetrics()
	publisher := events.NewPublisher(redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics), zerolog.Nop())
	// The event store is what makes published events observable from a test.
	publisher.SetEventStore(repos)
	svc := services.NewSessionService(repos, publisher, metrics, nil)

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "session_events")
	})

	ctx := context.Background()
	created, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := created.Session.ID

	if _, err := pool.Exec(ctx,
		`UPDATE sessions SET status = 'awaiting_reactivation', awaiting_reactivation_at = now() WHERE id = $1`,
		sessionID); err != nil {
		t.Fatalf("pause session: %v", err)
	}

	if _, err := svc.Reactivate(ctx, sessionID); err != nil {
		t.Fatalf("Reactivate: %v", err)
	}

	published := sessionEventsFor(t, pool, sessionID)
	if !eventsContain(published, "SESSION_REACTIVATED") {
		t.Errorf("published events %v: missing SESSION_REACTIVATED", published)
	}
	// Exactly one SESSION_CREATED — the real one from CreateSession. The
	// reactivation must not masquerade as a second create.
	creates := 0
	for _, e := range published {
		if e == "SESSION_CREATED" {
			creates++
		}
	}
	if creates != 1 {
		t.Errorf("published events %v: got %d SESSION_CREATED, want 1", published, creates)
	}

	// The payload must never carry the session row: sessions.session_token is a
	// guest credential and this event fans out to every device at the table.
	payload := sessionEventPayload(t, pool, sessionID, "SESSION_REACTIVATED")
	if payload["session_id"] != sessionID.String() {
		t.Errorf("payload session_id: got %v, want %s", payload["session_id"], sessionID)
	}
	if payload["status"] != "active" {
		t.Errorf("payload status: got %v, want active", payload["status"])
	}
	if _, leaked := payload["session_token"]; leaked {
		t.Error("SESSION_REACTIVATED payload leaks session_token")
	}
}

// F-27: SESSION_CREATED published CreateSessionResult whole, and sqlc.Session
// carries session_token — the guest credential every other surface strips
// (guestSafeSession, the snapshot handler, the support serializer). It went out
// over the session's WebSocket channel AND into event_log, which any staff
// member on the branch can read back via GET /sessions/{id}/events.
func TestSessionCreated_PayloadCarriesNoCredential(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	redisClient := openServiceTestRedis(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)

	metrics := observability.NewMetrics()
	publisher := events.NewPublisher(redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics), zerolog.Nop())
	publisher.SetEventStore(repos)
	svc := services.NewSessionService(repos, publisher, metrics, nil)

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "session_events", "event_log")
	})

	ctx := context.Background()
	created, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	// The caller still receives the real token — only what leaves the process is stripped.
	if created.Session.SessionToken == "" {
		t.Fatal("CreateSession result should still carry the token for the caller")
	}
	token := created.Session.SessionToken

	// Broadcast payload.
	payload := sessionEventPayload(t, pool, created.Session.ID, "SESSION_CREATED")
	session, ok := payload["session"].(map[string]any)
	if !ok {
		t.Fatalf("SESSION_CREATED payload has no `session` key (snake_case per F-27): %v", payload)
	}
	if _, ok := payload["participant"]; !ok {
		t.Errorf("SESSION_CREATED payload has no `participant` key: %v", payload)
	}
	if got := session["session_token"]; got != "" {
		t.Errorf("SESSION_CREATED broadcasts the guest credential: session_token=%v", got)
	}

	// Persisted event_log row — readable by any branch staff member.
	var logged []byte
	if err := pool.QueryRow(ctx,
		`SELECT payload FROM event_log WHERE session_id = $1 AND event_type = 'SESSION_CREATED' LIMIT 1`,
		created.Session.ID).Scan(&logged); err != nil {
		t.Fatalf("read event_log payload: %v", err)
	}
	if strings.Contains(string(logged), token) {
		t.Error("event_log SESSION_CREATED row contains the guest credential")
	}
}

func TestSnapshotReconnect_PublishesSessionReactivated(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	redisClient := openServiceTestRedis(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)

	metrics := observability.NewMetrics()
	publisher := events.NewPublisher(redisPkg.NewPubSub(redisClient, zerolog.Nop(), metrics), zerolog.Nop())
	publisher.SetEventStore(repos)
	svc := services.NewSessionService(repos, publisher, metrics, nil)

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "session_events")
	})

	ctx := context.Background()
	created, err := svc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := created.Session.ID

	if _, err := pool.Exec(ctx,
		`UPDATE sessions SET status = 'awaiting_reactivation', awaiting_reactivation_at = now() WHERE id = $1`,
		sessionID); err != nil {
		t.Fatalf("pause session: %v", err)
	}

	// This is the automatic path — the guest's client reconnects, the snapshot
	// endpoint reactivates as a side effect.
	snap, err := svc.GetSnapshot(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if string(snap.Session.Status) != "active" {
		t.Fatalf("snapshot status: got %s, want active", snap.Session.Status)
	}
	if published := sessionEventsFor(t, pool, sessionID); !eventsContain(published, "SESSION_REACTIVATED") {
		t.Errorf("published events %v: missing SESSION_REACTIVATED", published)
	}
}
