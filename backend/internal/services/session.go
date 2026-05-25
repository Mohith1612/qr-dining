package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type SessionService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	metrics   *observability.Metrics
}

func NewSessionService(repos *repository.Repos, publisher *events.Publisher, metrics *observability.Metrics) *SessionService {
	return &SessionService{repos: repos, publisher: publisher, metrics: metrics}
}

type CreateSessionResult struct {
	Session     sqlc.Session
	Participant sqlc.SessionParticipant
}

// CreateSession opens a new session for a table.
// It runs a single transaction that:
//  1. Verifies no active session exists for the table.
//  2. Inserts the session (host_participant_id = NULL).
//  3. Inserts the host participant (is_host = true).
//  4. Sets sessions.host_participant_id (DEFERRABLE FK committed at step 5).
//  5. Marks the table as occupied.
func (s *SessionService) CreateSession(ctx context.Context, tableID int64, displayName, deviceFingerprint string) (CreateSessionResult, error) {
	// Verify table is free before starting the transaction.
	_, err := s.repos.GetActiveSessionForTable(ctx, tableID)
	if err == nil {
		return CreateSessionResult{}, domain.ErrSessionAlreadyActive
	}
	if !errors.Is(err, domain.ErrSessionNotFound) {
		return CreateSessionResult{}, err
	}

	table, err := s.repos.GetTableByID(ctx, tableID)
	if err != nil {
		return CreateSessionResult{}, err
	}

	token, err := crypto.GenerateToken()
	if err != nil {
		return CreateSessionResult{}, fmt.Errorf("generate session token: %w", err)
	}

	var result CreateSessionResult

	err = s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		// Defer the FK check so we can insert session before participant.
		if err := tx.ExecRaw(ctx, "SET CONSTRAINTS fk_sessions_host_participant DEFERRED"); err != nil {
			return err
		}

		sess, err := tx.CreateSession(ctx, table.BranchID, tableID, token)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		participant, err := tx.CreateParticipant(ctx, sess.ID, displayName, deviceFingerprint, true)
		if err != nil {
			return fmt.Errorf("create host participant: %w", err)
		}

		if err := tx.SetSessionHost(ctx, sess.ID, participant.ID); err != nil {
			return fmt.Errorf("set session host: %w", err)
		}

		if err := tx.UpdateTableStatus(ctx, tableID, sqlc.TableStatusOccupied); err != nil {
			return fmt.Errorf("mark table occupied: %w", err)
		}

		result = CreateSessionResult{Session: sess, Participant: participant}
		return nil
	})
	if err != nil {
		return CreateSessionResult{}, err
	}

	s.publisher.SessionCreated(ctx, result.Session.ID, result)
	s.repos.LogEvent(ctx, result.Session.ID, result.Session.BranchID, "SESSION_CREATED", "participant", result.Participant.ID, result)
	return result, nil
}

// GetSession returns the session by ID. Returns ErrSessionNotFound if missing.
func (s *SessionService) GetSession(ctx context.Context, id uuid.UUID) (sqlc.Session, error) {
	return s.repos.GetSessionByID(ctx, id)
}

// SessionWithTable is a session enriched with the human-readable table identifier.
type SessionWithTable struct {
	sqlc.Session
	TableIdentifier string `json:"table_identifier"`
}

func (s *SessionService) ListActiveForBranch(ctx context.Context, branchID int64) ([]SessionWithTable, error) {
	sessions, err := s.repos.ListActiveSessionsForBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	result := make([]SessionWithTable, len(sessions))
	for i, sess := range sessions {
		swt := SessionWithTable{Session: sess}
		if t, err := s.repos.GetTableByID(ctx, sess.TableID); err == nil {
			swt.TableIdentifier = t.Identifier
		}
		result[i] = swt
	}
	return result, nil
}

// CloseSession closes an active session. Only the host participant may close it.
func (s *SessionService) CloseSession(ctx context.Context, id uuid.UUID, requesterID int64) error {
	sess, err := s.repos.GetSessionByID(ctx, id)
	if err != nil {
		return err
	}
	if sess.Status != sqlc.SessionStatusActive {
		return domain.ErrSessionClosed
	}
	if !sess.HostParticipantID.Valid || sess.HostParticipantID.Int64 != requesterID {
		return domain.ErrNotSessionHost
	}

	if err := s.repos.CloseSession(ctx, id); err != nil {
		return err
	}
	if err := s.repos.UpdateTableStatus(ctx, sess.TableID, sqlc.TableStatusAvailable); err != nil {
		return err
	}

	s.metrics.SessionDuration.Observe(time.Since(sess.CreatedAt).Seconds())

	s.publisher.SessionClosed(ctx, id, map[string]any{"session_id": id})
	s.repos.LogEvent(ctx, id, sess.BranchID, "SESSION_CLOSED", "participant", requesterID, map[string]any{"session_id": id})
	return nil
}

// JoinSession adds a participant to an existing active session.
func (s *SessionService) JoinSession(ctx context.Context, sessionID uuid.UUID, displayName, deviceFingerprint string) (sqlc.SessionParticipant, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sqlc.SessionParticipant{}, err
	}
	if sess.Status != sqlc.SessionStatusActive {
		return sqlc.SessionParticipant{}, domain.ErrSessionClosed
	}

	participant, err := s.repos.CreateParticipant(ctx, sessionID, displayName, deviceFingerprint, false)
	if err != nil {
		return sqlc.SessionParticipant{}, err
	}

	s.publisher.ParticipantJoined(ctx, sessionID, participant)
	s.repos.LogEvent(ctx, sessionID, sess.BranchID, "PARTICIPANT_JOINED", "participant", participant.ID, participant)
	return participant, nil
}

// SessionSnapshot is the full authoritative state of a session at a point in time.
// Clients call GET /sessions/:id/snapshot on WebSocket reconnect to reconcile local state.
type SessionSnapshot struct {
	Session         sqlc.Session             `json:"session"`
	TableIdentifier string                   `json:"table_identifier"`
	Participants    []sqlc.SessionParticipant `json:"participants"`
	Orders          []sqlc.Order             `json:"orders"`
	Assistance      []sqlc.AssistanceRequest `json:"assistance"`
	SnapshotAt      time.Time                `json:"snapshot_at"`
}

// GetSnapshot assembles the full current state of a session in parallel.
// Used by clients to reconcile state after a WebSocket reconnect.
func (s *SessionService) GetSnapshot(ctx context.Context, sessionID uuid.UUID) (SessionSnapshot, error) {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}

	var (
		participants    []sqlc.SessionParticipant
		orders          []sqlc.Order
		assistance      []sqlc.AssistanceRequest
		tableIdentifier string
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		participants, err = s.repos.ListParticipantsBySession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		var err error
		orders, err = s.repos.ListOrdersForSession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		var err error
		assistance, err = s.repos.ListAssistanceForSession(gctx, sessionID)
		return err
	})
	g.Go(func() error {
		if t, err := s.repos.GetTableByID(gctx, sess.TableID); err == nil {
			tableIdentifier = t.Identifier
		}
		return nil // non-fatal: fall back to numeric table_id on frontend
	})

	if err := g.Wait(); err != nil {
		return SessionSnapshot{}, err
	}

	return SessionSnapshot{
		Session:         sess,
		TableIdentifier: tableIdentifier,
		Participants:    participants,
		Orders:          orders,
		Assistance:      assistance,
		SnapshotAt:      time.Now().UTC(),
	}, nil
}

