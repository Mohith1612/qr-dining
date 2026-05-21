package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repos) CreateParticipant(ctx context.Context, sessionID uuid.UUID, displayName, deviceFingerprint string, isHost bool) (sqlc.SessionParticipant, error) {
	return r.q.CreateParticipant(ctx, sqlc.CreateParticipantParams{
		SessionID:         sessionID,
		DisplayName:       displayName,
		DeviceFingerprint: deviceFingerprint,
		IsHost:            isHost,
	})
}

func (r *Repos) GetParticipantByID(ctx context.Context, id int64) (sqlc.SessionParticipant, error) {
	p, err := r.q.GetParticipantByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SessionParticipant{}, domain.ErrParticipantNotFound
	}
	return p, err
}

func (r *Repos) ListParticipantsBySession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.SessionParticipant, error) {
	return r.q.ListParticipantsBySession(ctx, sessionID)
}

func (r *Repos) UpdateParticipantLastSeen(ctx context.Context, id int64) error {
	return r.q.UpdateParticipantLastSeen(ctx, id)
}
