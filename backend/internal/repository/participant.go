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

func (r *Repos) CreateParticipant(ctx context.Context, sessionID uuid.UUID, displayName, deviceFingerprint string, isHost bool, phoneE164 string) (sqlc.SessionParticipant, error) {
	return r.q.CreateParticipant(ctx, sqlc.CreateParticipantParams{
		SessionID:         sessionID,
		DisplayName:       displayName,
		DeviceFingerprint: deviceFingerprint,
		IsHost:            isHost,
		PhoneE164:         pgtype.Text{String: phoneE164, Valid: phoneE164 != ""},
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

// GetOldestActiveParticipant returns the earliest-joined non-revoked participant
// in a session — the successor when the host must be reassigned. Returns
// ErrParticipantNotFound if the session has no active participants.
func (r *Repos) GetOldestActiveParticipant(ctx context.Context, sessionID uuid.UUID) (sqlc.SessionParticipant, error) {
	p, err := r.q.GetOldestActiveParticipant(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SessionParticipant{}, domain.ErrParticipantNotFound
	}
	return p, err
}

// ReassignHost promotes newHostID to host: it flips is_host across the session's
// participants and points sessions.host_participant_id at the new host, in one
// transaction.
func (r *Repos) ReassignHost(ctx context.Context, sessionID uuid.UUID, newHostID int64) error {
	return r.WithTx(ctx, func(tx *Repos) error {
		if err := tx.q.SetParticipantHostFlags(ctx, sqlc.SetParticipantHostFlagsParams{
			SessionID: sessionID,
			ID:        newHostID,
		}); err != nil {
			return err
		}
		return tx.q.SetSessionHost(ctx, sqlc.SetSessionHostParams{
			ID:                sessionID,
			HostParticipantID: pgtype.Int8{Int64: newHostID, Valid: true},
		})
	})
}
