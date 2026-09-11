//go:build integration

package redis_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
)

func TestPresenceHeartbeatRetainsTimestampBeyondHostGrace(t *testing.T) {
	client := openTestRedis(t)
	presence := redisPkg.NewPresence(client)
	ctx := context.Background()
	sessionID := uuid.New()

	if err := presence.HeartbeatScoped(ctx, 11, 22, sessionID, 33); err != nil {
		t.Fatalf("HeartbeatScoped: %v", err)
	}
	key := fmt.Sprintf("org:%d:branch:%d:session:%s:presence", 11, 22, sessionID)
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 3*time.Minute {
		t.Fatalf("presence retention: got %s, want greater than 3m host grace", ttl)
	}
}

func TestPresenceGetPresentFiltersStaleFieldsIndependently(t *testing.T) {
	client := openTestRedis(t)
	presence := redisPkg.NewPresence(client)
	ctx := context.Background()
	sessionID := uuid.New()
	const (
		staleParticipantID = int64(33)
		liveParticipantID  = int64(44)
	)

	if err := presence.HeartbeatScoped(ctx, 11, 22, sessionID, staleParticipantID); err != nil {
		t.Fatalf("stale participant HeartbeatScoped: %v", err)
	}
	if err := presence.HeartbeatScoped(ctx, 11, 22, sessionID, liveParticipantID); err != nil {
		t.Fatalf("live participant HeartbeatScoped: %v", err)
	}
	key := fmt.Sprintf("org:%d:branch:%d:session:%s:presence", 11, 22, sessionID)
	if err := client.HSet(
		ctx,
		key,
		strconv.FormatInt(staleParticipantID, 10),
		time.Now().UTC().Add(-2*time.Minute).Format(time.RFC3339),
	).Err(); err != nil {
		t.Fatalf("backdate stale presence field: %v", err)
	}
	// Refreshing another field keeps the hash alive but must not keep the stale
	// participant logically present.
	if err := presence.HeartbeatScoped(ctx, 11, 22, sessionID, liveParticipantID); err != nil {
		t.Fatalf("refresh live participant: %v", err)
	}

	present, err := presence.GetPresentScoped(ctx, 11, 22, sessionID)
	if err != nil {
		t.Fatalf("GetPresentScoped: %v", err)
	}
	if _, ok := present[staleParticipantID]; ok {
		t.Fatalf("stale participant %d still reported present", staleParticipantID)
	}
	if _, ok := present[liveParticipantID]; !ok {
		t.Fatalf("live participant %d not reported present", liveParticipantID)
	}
}
