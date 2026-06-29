package events

import (
	"context"
	"time"

	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/redis"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Publisher wraps the Redis PubSub client and provides typed publish helpers.
// All service code publishes events through this instead of calling pubsub directly.
type Publisher struct {
	pubsub  *redis.PubSub
	store   EventStore
	logger  zerolog.Logger
	metrics *observability.Metrics
}

type EventStore interface {
	AppendSessionEvent(ctx context.Context, sessionID uuid.UUID, event ws.EventType, payload any) (ws.Envelope, error)
}

func NewPublisher(pubsub *redis.PubSub, logger zerolog.Logger) *Publisher {
	return &Publisher{pubsub: pubsub, logger: logger}
}

func (p *Publisher) SetEventStore(store EventStore) {
	p.store = store
}

// SetMetrics enables event-publish latency instrumentation. Optional and
// nil-safe so test publishers can skip it.
func (p *Publisher) SetMetrics(metrics *observability.Metrics) {
	p.metrics = metrics
}

// NewNoopPublisher returns a Publisher that discards all events. For tests only.
func NewNoopPublisher() *Publisher {
	return &Publisher{pubsub: nil, logger: zerolog.Nop()}
}

// publish is the internal helper — builds an Envelope and publishes to Redis.
// Errors are logged but not returned; event publish failures must not abort business operations.
func (p *Publisher) publish(ctx context.Context, event ws.EventType, sessionID uuid.UUID, payload any) {
	if p.pubsub == nil {
		return
	}
	start := time.Now()
	outcome := "ok"
	defer func() {
		if p.metrics != nil && p.metrics.EventPublishDuration != nil {
			p.metrics.EventPublishDuration.WithLabelValues(string(event), outcome).Observe(time.Since(start).Seconds())
		}
	}()

	var (
		env ws.Envelope
		err error
	)
	if p.store != nil {
		env, err = p.store.AppendSessionEvent(ctx, sessionID, event, payload)
		if err != nil {
			outcome = "append_error"
			p.logger.Error().Err(err).Str("event", string(event)).Str("session_id", sessionID.String()).Msg("failed to append session event")
			return
		}
	} else {
		env, err = ws.NewEnvelope(event, sessionID, payload)
		if err != nil {
			outcome = "encode_error"
			p.logger.Error().Err(err).Str("event", string(event)).Msg("failed to build event envelope")
			return
		}
	}
	p.logger.Debug().Str("event", string(event)).Str("session_id", sessionID.String()).Msg("publish event")
	if err := p.pubsub.Publish(ctx, env.SessionID, env.OrganizationID, env.BranchID, env); err != nil {
		outcome = "publish_error"
		p.logger.Error().Err(err).Str("event", string(event)).Msg("failed to publish event")
	}
}

// Typed publish methods — one per event type.

func (p *Publisher) SessionCreated(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventSessionCreated, sessionID, payload)
}

func (p *Publisher) ParticipantJoined(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventParticipantJoined, sessionID, payload)
}

func (p *Publisher) ParticipantLeft(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventParticipantLeft, sessionID, payload)
}

func (p *Publisher) CartUpdated(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventCartUpdated, sessionID, payload)
}

func (p *Publisher) OrderPlaced(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderPlaced, sessionID, payload)
}

func (p *Publisher) OrderConfirmed(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderConfirmed, sessionID, payload)
}

func (p *Publisher) OrderPreparing(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderPreparing, sessionID, payload)
}

func (p *Publisher) OrderReady(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderReady, sessionID, payload)
}

func (p *Publisher) OrderServed(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderServed, sessionID, payload)
}

func (p *Publisher) OrderCancelled(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventOrderCancelled, sessionID, payload)
}

func (p *Publisher) AssistanceRequested(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventAssistanceRequested, sessionID, payload)
}

func (p *Publisher) AssistanceAcknowledged(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventAssistanceAcknowledged, sessionID, payload)
}

func (p *Publisher) AssistanceResolved(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventAssistanceResolved, sessionID, payload)
}

func (p *Publisher) PaymentInitiated(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventPaymentInitiated, sessionID, payload)
}

func (p *Publisher) PaymentCompleted(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventPaymentCompleted, sessionID, payload)
}

func (p *Publisher) PaymentSettlementStalled(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventPaymentSettlementStalled, sessionID, payload)
}

func (p *Publisher) SessionClosed(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventSessionClosed, sessionID, payload)
}

func (p *Publisher) SessionExpiringSoon(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventSessionExpiringSoon, sessionID, payload)
}

func (p *Publisher) PromoApplied(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventPromoApplied, sessionID, payload)
}

func (p *Publisher) MenuItemAvailabilityChanged(ctx context.Context, sessionID uuid.UUID, payload any) {
	p.publish(ctx, ws.EventMenuItemAvailabilityChanged, sessionID, payload)
}
