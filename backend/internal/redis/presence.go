package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const presenceTTL = 90 * time.Second

// Presence tracks which session participants have active WebSocket connections.
// Stored as a Redis hash: presence:{session_id} → {participant_id: last_seen_timestamp}
// TTL is refreshed on each heartbeat. PostgreSQL remains the source of truth for
// participant records; Redis presence is a live view of who is currently connected.
type Presence struct {
	client *goredis.Client
}

func NewPresence(client *goredis.Client) *Presence {
	return &Presence{client: client}
}

// Heartbeat records a participant as present and refreshes the TTL.
func (p *Presence) Heartbeat(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	key := presenceKey(0, 0, sessionID)
	return p.heartbeat(ctx, key, participantID)
}

func (p *Presence) HeartbeatScoped(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID, participantID int64) error {
	key := presenceKey(organizationID, branchID, sessionID)
	return p.heartbeat(ctx, key, participantID)
}

func (p *Presence) heartbeat(ctx context.Context, key string, participantID int64) error {
	field := strconv.FormatInt(participantID, 10)
	now := time.Now().UTC().Format(time.RFC3339)

	pipe := p.client.Pipeline()
	pipe.HSet(ctx, key, field, now)
	pipe.Expire(ctx, key, presenceTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("presence heartbeat: %w", err)
	}
	return nil
}

// GetPresent returns a map of participantID → last_seen for a session.
func (p *Presence) GetPresent(ctx context.Context, sessionID uuid.UUID) (map[int64]time.Time, error) {
	raw, err := p.client.HGetAll(ctx, presenceKey(0, 0, sessionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get presence: %w", err)
	}

	result := make(map[int64]time.Time, len(raw))
	for idStr, tsStr := range raw {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		result[id] = ts
	}
	return result, nil
}

// Remove deletes a participant from the presence hash on disconnect.
func (p *Presence) Remove(ctx context.Context, sessionID uuid.UUID, participantID int64) error {
	field := strconv.FormatInt(participantID, 10)
	return p.client.HDel(ctx, presenceKey(0, 0, sessionID), field).Err()
}

// Delete removes the entire presence hash for a session (called on session close/abandon).
func (p *Presence) Delete(ctx context.Context, sessionID uuid.UUID) {
	p.client.Del(ctx, presenceKey(0, 0, sessionID))
}

func (p *Presence) DeleteScoped(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID) {
	p.client.Del(ctx, presenceKey(organizationID, branchID, sessionID), presenceKey(0, 0, sessionID))
}

func presenceKey(organizationID, branchID int64, sessionID uuid.UUID) string {
	if organizationID > 0 && branchID > 0 {
		return fmt.Sprintf("org:%d:branch:%d:session:%s:presence", organizationID, branchID, sessionID.String())
	}
	return "presence:" + sessionID.String()
}
