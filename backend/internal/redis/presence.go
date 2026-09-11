package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	// The browser sends an application heartbeat every 30 seconds. Three missed
	// heartbeats balance prompt absence detection against transient mobile drops.
	presenceStaleAfter      = 90 * time.Second
	defaultHostAbsenceGrace = 3 * time.Minute
)

// Presence tracks which session participants have active WebSocket connections.
// The primary hash is organization/branch scoped; a legacy unscoped hash is a
// fallback when scope metadata cannot be resolved. Readers union both. Hash TTL
// is refreshed on each heartbeat, while field age determines live presence.
// PostgreSQL remains the source of truth for participant records.
type Presence struct {
	client *goredis.Client
	keyTTL time.Duration
}

func NewPresence(client *goredis.Client) *Presence {
	return &Presence{
		client: client,
		keyTTL: defaultHostAbsenceGrace + presenceStaleAfter,
	}
}

// SetHostAbsenceGrace retains heartbeat timestamps long enough to evaluate the
// configured grace even after all clients stop refreshing a hash. It must be
// called at startup before heartbeats begin.
func (p *Presence) SetHostAbsenceGrace(grace time.Duration) {
	if grace > 0 {
		p.keyTTL = grace + presenceStaleAfter
	}
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
	pipe.Expire(ctx, key, p.keyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("presence heartbeat: %w", err)
	}
	return nil
}

// GetPresent returns a map of participantID → last_seen for a session.
func (p *Presence) GetPresent(ctx context.Context, sessionID uuid.UUID) (map[int64]time.Time, error) {
	lastSeen, err := p.GetLastSeen(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return filterPresent(lastSeen, time.Now().UTC(), presenceStaleAfter), nil
}

// GetLastSeen returns every valid participant timestamp in the legacy
// unscoped presence hash, including stale entries.
func (p *Presence) GetLastSeen(ctx context.Context, sessionID uuid.UUID) (map[int64]time.Time, error) {
	return p.getLastSeen(ctx, presenceKey(0, 0, sessionID), "get presence")
}

// GetPresentScoped reads the organization/branch-scoped presence hash and
// returns only participants seen within the current-presence threshold.
func (p *Presence) GetPresentScoped(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID) (map[int64]time.Time, error) {
	lastSeen, err := p.GetLastSeenScoped(ctx, organizationID, branchID, sessionID)
	if err != nil {
		return nil, err
	}
	return filterPresent(lastSeen, time.Now().UTC(), presenceStaleAfter), nil
}

// GetLastSeenScoped returns every valid participant timestamp in the scoped
// presence hash, including stale entries.
func (p *Presence) GetLastSeenScoped(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID) (map[int64]time.Time, error) {
	return p.getLastSeen(ctx, presenceKey(organizationID, branchID, sessionID), "get presence scoped")
}

// GetPresentForSession returns the age-filtered union of the scoped presence
// hash and the legacy fallback hash. All session-level readers use this view so
// a successful fallback heartbeat cannot become invisible to one subsystem.
func (p *Presence) GetPresentForSession(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID) (map[int64]time.Time, error) {
	scoped, err := p.GetPresentScoped(ctx, organizationID, branchID, sessionID)
	if err != nil {
		return nil, err
	}
	unscoped, err := p.GetPresent(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return mergeLastSeen(scoped, unscoped), nil
}

// GetLastSeenForSession returns the unfiltered union used when evaluating the
// longer host-absence grace period.
func (p *Presence) GetLastSeenForSession(ctx context.Context, organizationID, branchID int64, sessionID uuid.UUID) (map[int64]time.Time, error) {
	scoped, err := p.GetLastSeenScoped(ctx, organizationID, branchID, sessionID)
	if err != nil {
		return nil, err
	}
	unscoped, err := p.GetLastSeen(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return mergeLastSeen(scoped, unscoped), nil
}

func (p *Presence) getLastSeen(ctx context.Context, key, operation string) (map[int64]time.Time, error) {
	raw, err := p.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
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

func filterPresent(lastSeen map[int64]time.Time, now time.Time, staleAfter time.Duration) map[int64]time.Time {
	present := make(map[int64]time.Time, len(lastSeen))
	for id, seenAt := range lastSeen {
		if now.Sub(seenAt) <= staleAfter {
			present[id] = seenAt
		}
	}
	return present
}

func mergeLastSeen(sets ...map[int64]time.Time) map[int64]time.Time {
	merged := map[int64]time.Time{}
	for _, set := range sets {
		for id, seenAt := range set {
			if current, ok := merged[id]; !ok || seenAt.After(current) {
				merged[id] = seenAt
			}
		}
	}
	return merged
}

// TryThrottle returns true when the caller has won the throttle window for
// key (SETNX with ttl). Fail-open: a Redis error also returns true so the
// throttled action (e.g. a DB last_seen write) still happens.
func (p *Presence) TryThrottle(ctx context.Context, key string, ttl time.Duration) bool {
	ok, err := p.client.SetNX(ctx, key, "", ttl).Result()
	if err != nil {
		return true
	}
	return ok
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
