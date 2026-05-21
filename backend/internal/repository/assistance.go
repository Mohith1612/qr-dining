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

func (r *Repos) ListAssistanceForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.AssistanceRequest, error) {
	return r.q.ListAssistanceForSession(ctx, sessionID)
}

func (r *Repos) ListActiveAssistanceForBranch(ctx context.Context, branchID int64) ([]sqlc.AssistanceRequest, error) {
	return r.q.ListActiveAssistanceForBranch(ctx, branchID)
}
