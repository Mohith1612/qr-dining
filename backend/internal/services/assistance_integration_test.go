//go:build integration

package services_test

import (
	"context"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/testutil"
)

func TestAssistanceLifecycle(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	assistanceSvc := newTestAssistanceService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "assistance_requests")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ar, err := assistanceSvc.Request(ctx, sess.Session.ID, f.TableID, sess.Participant.ID, sqlc.AssistanceTypeWaiter)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if ar.Status != sqlc.AssistanceStatusPending {
		t.Errorf("initial status: got %q, want pending", ar.Status)
	}

	const staffID int64 = 1
	acked, err := assistanceSvc.Acknowledge(ctx, ar.ID, f.BranchID, staffID)
	if err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if acked.Status != sqlc.AssistanceStatusAcknowledged {
		t.Errorf("after ack: got %q, want acknowledged", acked.Status)
	}

	resolved, err := assistanceSvc.Resolve(ctx, ar.ID, f.BranchID, staffID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Status != sqlc.AssistanceStatusResolved {
		t.Errorf("after resolve: got %q, want resolved", resolved.Status)
	}
	if !resolved.ResolvedAt.Valid {
		t.Error("resolved_at should be set after resolve")
	}
}

func TestAssistance_InvalidTransition(t *testing.T) {
	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	pub := events.NewNoopPublisher()
	sessionSvc := newTestSessionService(repos, pub)
	assistanceSvc := newTestAssistanceService(repos, pub)
	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "sessions", "session_participants", "assistance_requests")
	})

	ctx := context.Background()
	sess, err := sessionSvc.CreateSession(ctx, f.TableID, "Alice", "fp-alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ar, err := assistanceSvc.Request(ctx, sess.Session.ID, f.TableID, sess.Participant.ID, sqlc.AssistanceTypeBill)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	// pending → resolved is a valid transition (staff may resolve directly,
	// matching the state machine in domain/statemachine.go).
	if _, err := assistanceSvc.Resolve(ctx, ar.ID, f.BranchID, 1); err != nil {
		t.Fatalf("Resolve (pending→resolved): %v", err)
	}

	// resolved is terminal: acknowledging an already-resolved request is invalid.
	_, err = assistanceSvc.Acknowledge(ctx, ar.ID, f.BranchID, 1)
	if err == nil {
		t.Fatal("expected ErrInvalidAssistanceTransition, got nil")
	}
	if !isErr(err, domain.ErrInvalidAssistanceTransition) {
		t.Errorf("expected ErrInvalidAssistanceTransition, got %v", err)
	}
}
