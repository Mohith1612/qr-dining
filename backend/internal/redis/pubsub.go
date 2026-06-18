package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const legacySessionChannelPrefix = "session:"
const scopedSessionChannelPrefix = "org:"
const sessionChannelSuffix = ":events"

// PubSub wraps Redis pub/sub for session event fanout.
// A single PSubscribe("session:*:events") handles all sessions without
// opening one subscription per active session.
type PubSub struct {
	client  *goredis.Client
	logger  zerolog.Logger
	metrics *observability.Metrics
}

func NewPubSub(client *goredis.Client, logger zerolog.Logger, metrics *observability.Metrics) *PubSub {
	return &PubSub{client: client, logger: logger, metrics: metrics}
}

// Publish encodes the payload as JSON and publishes it to the scoped session channel.
func (ps *PubSub) Publish(ctx context.Context, sessionID uuid.UUID, organizationID, branchID int64, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	channel := sessionChannel(sessionID, organizationID, branchID)
	if err := ps.client.Publish(ctx, channel, data).Err(); err != nil {
		ps.metrics.RedisPubSubErrors.WithLabelValues("publish").Inc()
		return fmt.Errorf("publish to %s: %w", channel, err)
	}
	return nil
}

// Message is received by the Hub's subscriber goroutine and routed to the correct room.
type Message struct {
	SessionID uuid.UUID
	Data      []byte
}

// Subscribe opens a pattern subscription for all session channels and delivers
// decoded messages to the provided channel. Blocks until ctx is cancelled.
// Intended to run as a dedicated goroutine started by the WebSocket Hub.
func (ps *PubSub) Subscribe(ctx context.Context, out chan<- Message) error {
	sub := ps.client.PSubscribe(ctx, legacySessionChannelPrefix+"*"+sessionChannelSuffix, scopedSessionChannelPrefix+"*"+sessionChannelSuffix)
	defer func() {
		sub.Close()
		ps.metrics.RedisPubSubConnected.Set(0)
	}()

	ps.metrics.RedisPubSubConnected.Set(1)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				ps.metrics.RedisPubSubErrors.WithLabelValues("receive").Inc()
				return nil
			}
			sessionID, err := parseSessionIDFromChannel(msg.Channel)
			if err != nil {
				ps.logger.Warn().Str("channel", msg.Channel).Msg("unparseable pubsub channel")
				continue
			}
			select {
			case out <- Message{SessionID: sessionID, Data: []byte(msg.Payload)}:
			default:
				ps.logger.Warn().Str("session_id", sessionID.String()).Msg("pubsub dispatch channel full; dropping message")
			}
		}
	}
}

func sessionChannel(sessionID uuid.UUID, organizationID, branchID int64) string {
	if organizationID > 0 && branchID > 0 {
		return fmt.Sprintf("org:%d:branch:%d:session:%s%s", organizationID, branchID, sessionID.String(), sessionChannelSuffix)
	}
	return legacySessionChannelPrefix + sessionID.String() + sessionChannelSuffix
}

func parseSessionIDFromChannel(channel string) (uuid.UUID, error) {
	if strings.HasPrefix(channel, legacySessionChannelPrefix) {
		s := strings.TrimPrefix(channel, legacySessionChannelPrefix)
		s = strings.TrimSuffix(s, sessionChannelSuffix)
		return uuid.Parse(s)
	}
	parts := strings.Split(channel, ":")
	if len(parts) >= 6 && parts[0] == "org" && parts[2] == "branch" && parts[4] == "session" {
		return uuid.Parse(parts[5])
	}
	s := strings.TrimPrefix(channel, legacySessionChannelPrefix)
	s = strings.TrimSuffix(s, sessionChannelSuffix)
	return uuid.Parse(s)
}
