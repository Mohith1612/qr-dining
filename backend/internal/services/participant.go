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
	"github.com/rs/zerolog"
)

const presenceDBSyncInterval = 2 * time.Minute

type ParticipantService struct {
	repos     *repository.Repos
	publisher *events.Publisher
	presence  *redisPkg.Presence
	logger    zerolog.Logger
}

func NewParticipantService(repos *repository.Repos, publisher *events.Publisher, presence *redisPkg.Presence) *ParticipantService {
	return &ParticipantService{repos: repos, publisher: publisher, presence: presence, logger: zerolog.Nop()}
}

// SetLogger wires fallback observability. It must be called at startup before
// the service begins handling heartbeats.
func (s *ParticipantService) SetLogger(logger zerolog.Logger) { s.logger = logger }

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
// ~30s). Redis is refreshed on every call; current presence allows three missed
// pings (90s), while the hash itself is retained through the host-transfer grace.
// The DB last_seen_at write is throttled to one per participant per ~2 minutes
// via SETNX so a busy room doesn't turn pings into row churn.
func (s *ParticipantService) UpdatePresenceThrottled(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	if s.presence.TryThrottle(ctx, fmt.Sprintf("presence_dbsync:%d", participantID), presenceDBSyncInterval) {
		if err := s.repos.UpdateParticipantLastSeen(ctx, participantID); err != nil {
			return err
		}
	}
	return s.heartbeatPresence(ctx, sessionID, participantID)
}

func (s *ParticipantService) heartbeatPresence(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	sess, err := s.repos.GetSessionByID(ctx, sessionID)
	if err != nil {
		s.warnUnscopedFallback(sessionID, participantID, "session", err)
		return s.presence.Heartbeat(ctx, sessionID, participantID)
	}
	org, err := s.repos.GetOrganizationByBranchID(ctx, sess.BranchID)
	if err != nil {
		s.warnUnscopedFallback(sessionID, participantID, "organization", err)
		return s.presence.Heartbeat(ctx, sessionID, participantID)
	}
	return s.presence.HeartbeatScoped(ctx, org.ID, sess.BranchID, sessionID, participantID)
}

func (s *ParticipantService) warnUnscopedFallback(sessionID uuid.UUID, participantID int64, lookup string, err error) {
	// Retire the legacy key path once this warning remains at zero in production.
	s.logger.Warn().
		Err(err).
		Str("session_id", sessionID.String()).
		Int64("participant_id", participantID).
		Str("failed_lookup", lookup).
		Msg("presence heartbeat using legacy unscoped fallback")
}
