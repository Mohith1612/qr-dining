package services

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/repository"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
)

type ParticipantService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	presence  *redisPkg.Presence
}

func NewParticipantService(repos *repository.Repos, publisher *events.Publisher, presence *redisPkg.Presence) *ParticipantService {
	return &ParticipantService{repos: repos, publisher: publisher, presence: presence}
}

func (s *ParticipantService) GetByID(ctx context.Context, id int64) (sqlc.SessionParticipant, error) {
	return s.repos.GetParticipantByID(ctx, id)
}

func (s *ParticipantService) ListBySession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.SessionParticipant, error) {
	return s.repos.ListParticipantsBySession(ctx, sessionID)
}

// UpdatePresence refreshes the participant's last_seen_at in the DB and sets the Redis presence key.
func (s *ParticipantService) UpdatePresence(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	if err := s.repos.UpdateParticipantLastSeen(ctx, participantID); err != nil {
		return err
	}
	return s.presence.Heartbeat(ctx, sessionID, participantID)
}
