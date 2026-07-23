package websocket

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// refreshPresence must invoke the injected refresher with the client's
// identity and be a silent no-op when none is wired (nil-safety matters: the
// refresher is optional and readPump calls this on every client PING).
func TestRefreshPresenceInvokesInjectedRefresher(t *testing.T) {
	h := NewHub(nil, nil, zerolog.Nop(), nil)

	sessionID := uuid.New()
	var gotSession uuid.UUID
	var gotParticipant int64
	calls := 0
	h.SetPresenceRefresher(func(ctx context.Context, sid uuid.UUID, pid int64) {
		calls++
		gotSession = sid
		gotParticipant = pid
		if _, ok := ctx.Deadline(); !ok {
			t.Error("refresher context must carry a deadline")
		}
	})

	h.refreshPresence(sessionID, 42)
	if calls != 1 {
		t.Fatalf("refresher calls = %d, want 1", calls)
	}
	if gotSession != sessionID || gotParticipant != 42 {
		t.Fatalf("refresher got (%s, %d), want (%s, 42)", gotSession, gotParticipant, sessionID)
	}
}

func TestRefreshPresenceNilSafe(t *testing.T) {
	h := NewHub(nil, nil, zerolog.Nop(), nil)
	// No refresher wired — must not panic.
	h.refreshPresence(uuid.New(), 7)
}
