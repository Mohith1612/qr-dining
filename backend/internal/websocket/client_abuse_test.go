package websocket

import (
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestTokenBucketCapsSustainedFlood(t *testing.T) {
	var b tokenBucket
	now := time.Now()

	// A fresh bucket allows a full burst, then drops.
	allowed := 0
	for i := 0; i < inboundBurst+10; i++ {
		if b.allow(now) {
			allowed++
		}
	}
	if allowed != inboundBurst {
		t.Fatalf("expected %d frames allowed in initial burst, got %d", inboundBurst, allowed)
	}

	// After one second the bucket refills by inboundRefillPerSec.
	now = now.Add(time.Second)
	refilled := 0
	for i := 0; i < inboundBurst; i++ {
		if b.allow(now) {
			refilled++
		}
	}
	if refilled != int(inboundRefillPerSec) {
		t.Fatalf("expected %d frames after 1s refill, got %d", int(inboundRefillPerSec), refilled)
	}
}

func TestRecordStrikeForceClosesAfterBudget(t *testing.T) {
	m := observability.NewMetrics()
	c := &Client{
		hub:       &Hub{metrics: m, logger: zerolog.Nop()},
		logger:    zerolog.Nop(),
		sessionID: uuid.New(),
	}

	for i := 1; i < maxInboundStrikes; i++ {
		if c.recordStrike("inbound_rate_exceeded") {
			t.Fatalf("connection force-closed too early at strike %d", i)
		}
	}
	if !c.recordStrike("inbound_rate_exceeded") {
		t.Fatalf("expected force-close at strike %d", maxInboundStrikes)
	}
}
