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

func (r *Repos) CreateAssistanceRequest(ctx context.Context, sessionID uuid.UUID, tableID, participantID int64, reqType sqlc.AssistanceType) (sqlc.AssistanceRequest, error) {
	return r.q.CreateAssistanceRequest(ctx, sqlc.CreateAssistanceRequestParams{
		SessionID:     sessionID,
		TableID:       tableID,
		ParticipantID: pgtype.Int8{Int64: participantID, Valid: participantID != 0},
		Type:          reqType,
	})
}

func (r *Repos) GetAssistanceRequestByID(ctx context.Context, id int64) (sqlc.AssistanceRequest, error) {
	a, err := r.q.GetAssistanceRequestByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AssistanceRequest{}, domain.ErrAssistanceNotFound
	}
	return a, err
}

func (r *Repos) UpdateAssistanceStatus(ctx context.Context, id int64, status sqlc.AssistanceStatus) (sqlc.AssistanceRequest, error) {
	a, err := r.q.UpdateAssistanceStatus(ctx, sqlc.UpdateAssistanceStatusParams{
		ID:     id,
		Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AssistanceRequest{}, domain.ErrAssistanceNotFound
	}
	return a, err
}

func (r *Repos) UpdateAssistanceStatusScoped(ctx context.Context, id, branchID int64, status sqlc.AssistanceStatus) (sqlc.AssistanceRequest, error) {
	row := r.db.QueryRow(ctx, `
UPDATE assistance_requests ar
SET status = $3,
    resolved_at = CASE WHEN $3::assistance_status = 'resolved' THEN NOW() ELSE ar.resolved_at END
FROM sessions s
WHERE ar.id = $1
  AND ar.session_id = s.id
  AND s.branch_id = $2
RETURNING ar.id, ar.session_id, ar.table_id, ar.participant_id, ar.type, ar.status, ar.created_at, ar.resolved_at
`, id, branchID, status)
	var a sqlc.AssistanceRequest
	err := row.Scan(
		&a.ID,
		&a.SessionID,
		&a.TableID,
		&a.ParticipantID,
		&a.Type,
		&a.Status,
		&a.CreatedAt,
		&a.ResolvedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AssistanceRequest{}, domain.ErrAssistanceNotFound
	}
	return a, err
}

func (r *Repos) ListAssistanceForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.AssistanceRequest, error) {
	return r.q.ListAssistanceForSession(ctx, sessionID)
}

func (r *Repos) ListActiveAssistanceForBranch(ctx context.Context, branchID int64) ([]sqlc.AssistanceRequest, error) {
	return r.q.ListActiveAssistanceForBranch(ctx, branchID)
}
