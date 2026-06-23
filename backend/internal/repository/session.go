package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) CreateSession(ctx context.Context, p sqlc.CreateSessionParams) (sqlc.Session, error) {
	sess, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		BranchID:            p.BranchID,
		TableID:             p.TableID,
		SessionToken:        p.SessionToken,
		SessionBusinessDate: p.SessionBusinessDate,
		VisitNumber:         p.VisitNumber,
		SessionNumber:       p.SessionNumber,
	})
	if isDuplicateError(err) {
		return sqlc.Session{}, domain.ErrSessionAlreadyActive
	}
	return sess, err
}

func (r *Repos) NextSessionNumber(ctx context.Context, branchID int64, businessDate pgtype.Date) (int32, error) {
	return r.q.NextSessionNumber(ctx, sqlc.NextSessionNumberParams{
		BranchID: branchID,
		Date:     businessDate,
	})
}

func (r *Repos) SetSessionHost(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	return r.q.SetSessionHost(ctx, sqlc.SetSessionHostParams{
		ID:                sessionID,
		HostParticipantID: pgtype.Int8{Int64: participantID, Valid: true},
	})
}

func (r *Repos) GetSessionByID(ctx context.Context, id uuid.UUID) (sqlc.Session, error) {
	s, err := r.q.GetSessionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Session{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *Repos) GetSessionParticipantByID(ctx context.Context, id int64) (sqlc.SessionParticipant, error) {
	p, err := r.q.GetSessionParticipantByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SessionParticipant{}, domain.ErrParticipantNotFound
	}
	return p, err
}

func (r *Repos) GetSessionByToken(ctx context.Context, token string) (sqlc.Session, error) {
	s, err := r.q.GetSessionByToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Session{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *Repos) GetActiveSessionForTable(ctx context.Context, tableID int64) (sqlc.Session, error) {
	s, err := r.q.GetActiveSessionForTable(ctx, tableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Session{}, domain.ErrSessionNotFound
	}
	return s, err
}

// CloseSessionIfActive closes the session only if it's currently active.
// Returns pgx.ErrNoRows if the session was already closed — callers treat that as idempotent success.
func (r *Repos) CloseSessionIfActive(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return r.q.CloseSessionIfActive(ctx, id)
}

func (r *Repos) AbandonSession(ctx context.Context, id uuid.UUID) error {
	return r.q.AbandonSession(ctx, id)
}

// TransitionSessionToPaymentPending moves an active session into payment_pending
// inside the caller's transaction. Returns pgx.ErrNoRows if the row is not in
// 'active' status — callers treat that as a "session already in a non-active
// state" signal and decide whether to fail or merge.
func (r *Repos) TransitionSessionToPaymentPending(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return r.q.TransitionSessionToPaymentPending(ctx, id)
}

// TransitionSessionToActive reverses payment_pending back to active when every
// payment against the bill snapshot has terminally failed or been cancelled.
func (r *Repos) TransitionSessionToActive(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return r.q.TransitionSessionToActive(ctx, id)
}

// RevokeAllParticipants marks every non-revoked participant in a session as
// revoked. Used inside the close/abandon transactions so stored guest tokens
// can never be used to act on a terminal session.
func (r *Repos) RevokeAllParticipants(ctx context.Context, sessionID uuid.UUID, reason string) error {
	return r.q.RevokeAllParticipants(ctx, sqlc.RevokeAllParticipantsParams{
		SessionID:     sessionID,
		RevokedReason: pgtype.Text{String: reason, Valid: reason != ""},
	})
}

// BumpAllParticipantCredentialVersions increments credential_version for every
// participant in a session. Guest tokens issued with the prior version no
// longer validate, achieving total credential invalidation without a separate
// JTI denylist.
func (r *Repos) BumpAllParticipantCredentialVersions(ctx context.Context, sessionID uuid.UUID) error {
	return r.q.BumpAllParticipantCredentialVersions(ctx, sessionID)
}

func (r *Repos) ListActiveSessionsForBranch(ctx context.Context, branchID int64) ([]sqlc.Session, error) {
	return r.q.ListActiveSessionsForBranch(ctx, branchID)
}

func (r *Repos) AppendSessionEvent(ctx context.Context, sessionID uuid.UUID, event ws.EventType, payload any) (ws.Envelope, error) {
	env, err := ws.NewEnvelope(event, sessionID, payload)
	if err != nil {
		return ws.Envelope{}, err
	}

	raw := []byte("{}")
	if env.Payload != nil {
		raw = env.Payload
	}

	err = r.WithTx(ctx, func(tx *Repos) error {
		if err := tx.ExecRaw(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, sessionID.String()); err != nil {
			return err
		}

		row := tx.db.QueryRow(ctx, `
WITH next_sequence AS (
    SELECT COALESCE(MAX(sequence), 0) + 1 AS sequence
    FROM session_events
    WHERE session_id = $1
),
session_scope AS (
    SELECT s.branch_id, b.organization_id
    FROM sessions s
    JOIN branches b ON b.id = s.branch_id
    WHERE s.id = $1
)
INSERT INTO session_events (id, sequence, organization_id, branch_id, session_id, event, payload)
SELECT $2, next_sequence.sequence, session_scope.organization_id, session_scope.branch_id, $1, $3, $4::jsonb
FROM next_sequence, session_scope
RETURNING sequence, organization_id, branch_id, created_at
`, sessionID, env.EventID, string(event), raw)
		if err := row.Scan(&env.Sequence, &env.OrganizationID, &env.BranchID, &env.Timestamp); err != nil {
			return fmt.Errorf("append session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return ws.Envelope{}, err
	}

	// Normalize through JSON so callers always publish a compact object payload.
	if env.Payload == nil {
		env.Payload = json.RawMessage("{}")
	}
	return env, nil
}

func (r *Repos) ListSessionEventsAfter(ctx context.Context, sessionID uuid.UUID, lastSequence int64) ([]ws.Envelope, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, sequence, organization_id, branch_id, session_id, event, payload, created_at
FROM session_events
WHERE session_id = $1 AND sequence > $2
ORDER BY sequence ASC
LIMIT 500
`, sessionID, lastSequence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []ws.Envelope{}
	for rows.Next() {
		var env ws.Envelope
		var event string
		if err := rows.Scan(
			&env.EventID,
			&env.Sequence,
			&env.OrganizationID,
			&env.BranchID,
			&env.SessionID,
			&event,
			&env.Payload,
			&env.Timestamp,
		); err != nil {
			return nil, err
		}
		env.Event = ws.EventType(event)
		events = append(events, env)
	}
	return events, rows.Err()
}
