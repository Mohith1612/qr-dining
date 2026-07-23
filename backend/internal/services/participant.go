package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/events"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
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
	return s.heartbeatPresence(ctx, sessionID, participantID)
}

// UpdatePresenceThrottled is the WebSocket heartbeat path (client PING every
// ~30s). The Redis presence hash is refreshed on every call (TTL 90s → 3×
// margin); the DB last_seen_at write is throttled to one per participant per
// ~2 minutes via SETNX so a busy room doesn't turn pings into row churn.
func (s *ParticipantService) UpdatePresenceThrottled(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	if s.presence.TryThrottle(ctx, fmt.Sprintf("presence_dbsync:%d", participantID), 2*time.Minute) {
		if err := s.repos.UpdateParticipantLastSeen(ctx, participantID); err != nil {
			return err
		}
	}
	return s.heartbeatPresence(ctx, sessionID, participantID)
}

func (s *ParticipantService) heartbeatPresence(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err == nil {
		if org, err := s.repos.GetOrganizationByBranchID(ctx, sess.BranchID); err == nil {
			return s.presence.HeartbeatScoped(ctx, org.ID, sess.BranchID, sessionID, participantID)
		}
	}
	return s.presence.Heartbeat(ctx, sessionID, participantID)
}
