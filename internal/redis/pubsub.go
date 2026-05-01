package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const sessionChannelPrefix = "session:"
const sessionChannelSuffix = ":events"

// PubSub wraps Redis pub/sub for session event fanout.
// A single PSubscribe("session:*:events") handles all sessions without
// opening one subscription per active session.
type PubSub struct {
	client *goredis.Client
	logger zerolog.Logger
}

func NewPubSub(client *goredis.Client, logger zerolog.Logger) *PubSub {
	return &PubSub{client: client, logger: logger}
}

// Publish encodes the payload as JSON and publishes it to the session's channel.
func (ps *PubSub) Publish(ctx context.Context, sessionID uuid.UUID, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	channel := sessionChannel(sessionID)
	if err := ps.client.Publish(ctx, channel, data).Err(); err != nil {
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
	sub := ps.client.PSubscribe(ctx, sessionChannelPrefix+"*"+sessionChannelSuffix)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
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

func sessionChannel(id uuid.UUID) string {
	return sessionChannelPrefix + id.String() + sessionChannelSuffix
}

func parseSessionIDFromChannel(channel string) (uuid.UUID, error) {
	s := strings.TrimPrefix(channel, sessionChannelPrefix)
	s = strings.TrimSuffix(s, sessionChannelSuffix)
	return uuid.Parse(s)
}
