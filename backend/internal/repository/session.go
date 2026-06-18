package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) CreateSession(ctx context.Context, branchID, tableID int64, token string) (sqlc.Session, error) {
	return r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		BranchID:     branchID,
		TableID:      tableID,
		SessionToken: token,
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

func (r *Repos) ListActiveSessionsForBranch(ctx context.Context, branchID int64) ([]sqlc.Session, error) {
	return r.q.ListActiveSessionsForBranch(ctx, branchID)
}
