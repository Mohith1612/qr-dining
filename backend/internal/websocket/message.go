package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType is the discriminator field for all WebSocket events.
type EventType string

const (
	EventSessionCreated         EventType = "SESSION_CREATED"
	EventParticipantJoined      EventType = "PARTICIPANT_JOINED"
	EventParticipantLeft        EventType = "PARTICIPANT_LEFT"
	EventItemAdded              EventType = "ITEM_ADDED"
	EventItemRemoved            EventType = "ITEM_REMOVED"
	EventCartUpdated            EventType = "CART_UPDATED"
	EventOrderPlaced            EventType = "ORDER_PLACED"
	EventOrderConfirmed         EventType = "ORDER_CONFIRMED"
	EventOrderPreparing         EventType = "ORDER_PREPARING"
	EventOrderReady             EventType = "ORDER_READY"
	EventOrderServed            EventType = "ORDER_SERVED"
	EventOrderCancelled         EventType = "ORDER_CANCELLED"
	EventAssistanceRequested    EventType = "ASSISTANCE_REQUESTED"
	EventAssistanceAcknowledged EventType = "ASSISTANCE_ACKNOWLEDGED"
	EventAssistanceResolved     EventType = "ASSISTANCE_RESOLVED"
	EventPaymentInitiated       EventType = "PAYMENT_INITIATED"
	EventPaymentCompleted       EventType = "PAYMENT_COMPLETED"
	// EventPaymentCancelled tells live guests a payment request was withdrawn
	// by staff. The payload carries the session status alongside the payment,
	// because cancelling the last non-terminal payment releases the
	// payment_pending freeze and the cart becomes writable again.
	EventPaymentCancelled EventType = "PAYMENT_CANCELLED"
	// EventPaymentSettlementStalled surfaces a payment_pending session whose
	// settlement has stalled past the escalation threshold. Emitted by the
	// escalation worker for operator visibility; no state change implied.
	EventPaymentSettlementStalled EventType = "PAYMENT_SETTLEMENT_STALLED"
	EventSessionClosed            EventType = "SESSION_CLOSED"
	EventSessionExpiringSoon      EventType = "SESSION_EXPIRING_SOON"
	// EventSessionReactivated tells live guests an idled session moved from
	// awaiting_reactivation back to active, so they can drop the "table paused"
	// overlay. This used to be published as SESSION_CREATED, which no client
	// handler could act on and which collided with the genuine create event's
	// payload shape — leaving the reactivation banner up over a live session
	// (F-08). The payload is deliberately minimal: never the session row, which
	// carries the guest credential (session_token).
	EventSessionReactivated EventType = "SESSION_REACTIVATED"
	EventPromoApplied       EventType = "PROMO_APPLIED"
	// EventHostChanged tells live participants the session host was reassigned
	// (the previous host left/was lost). Payload is the new host participant so
	// clients can update the host badge and re-evaluate host-only affordances.
	EventHostChanged EventType = "HOST_CHANGED"
	// EventMenuItemAvailabilityChanged tells live guests a menu item was
	// enabled/disabled mid-session so they can reconcile their menu and cart
	// instead of failing an order against a now-unavailable item.
	EventMenuItemAvailabilityChanged EventType = "MENU_ITEM_AVAILABILITY_CHANGED"
	EventPing                        EventType = "PING"
	EventPong                        EventType = "PONG"
)

// Envelope is the standard shape for all WebSocket messages.
// Clients must treat events as at-least-once; they can deduplicate by
// (event, session_id, timestamp) if needed.
type Envelope struct {
	EventID        uuid.UUID       `json:"event_id"`
	Sequence       int64           `json:"sequence"`
	OrganizationID int64           `json:"organization_id"`
	BranchID       int64           `json:"branch_id"`
	SessionID      uuid.UUID       `json:"session_id"`
	Event          EventType       `json:"event"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// NewEnvelope constructs an Envelope, marshalling the payload into RawMessage.
func NewEnvelope(event EventType, sessionID uuid.UUID, payload any) (Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return Envelope{}, err
		}
	}
	return Envelope{
		EventID:   uuid.New(),
		Event:     event,
		SessionID: sessionID,
		Payload:   raw,
		Timestamp: time.Now().UTC(),
	}, nil
}
