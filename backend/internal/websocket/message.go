package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType is the discriminator field for all WebSocket events.
type EventType string

const (
	EventSessionCreated          EventType = "SESSION_CREATED"
	EventParticipantJoined       EventType = "PARTICIPANT_JOINED"
	EventParticipantLeft         EventType = "PARTICIPANT_LEFT"
	EventItemAdded               EventType = "ITEM_ADDED"
	EventItemRemoved             EventType = "ITEM_REMOVED"
	EventCartUpdated             EventType = "CART_UPDATED"
	EventOrderPlaced             EventType = "ORDER_PLACED"
	EventOrderConfirmed          EventType = "ORDER_CONFIRMED"
	EventOrderPreparing          EventType = "ORDER_PREPARING"
	EventOrderReady              EventType = "ORDER_READY"
	EventOrderServed             EventType = "ORDER_SERVED"
	EventOrderCancelled          EventType = "ORDER_CANCELLED"
	EventAssistanceRequested     EventType = "ASSISTANCE_REQUESTED"
	EventAssistanceAcknowledged  EventType = "ASSISTANCE_ACKNOWLEDGED"
	EventAssistanceResolved      EventType = "ASSISTANCE_RESOLVED"
	EventPaymentInitiated        EventType = "PAYMENT_INITIATED"
	EventPaymentCompleted        EventType = "PAYMENT_COMPLETED"
	EventSessionClosed           EventType = "SESSION_CLOSED"
	EventSessionExpiringSoon     EventType = "SESSION_EXPIRING_SOON"
	EventPromoApplied            EventType = "PROMO_APPLIED"
	EventPing                    EventType = "PING"
	EventPong                    EventType = "PONG"
)

// Envelope is the standard shape for all WebSocket messages.
// Clients must treat events as at-least-once; they can deduplicate by
// (event, session_id, timestamp) if needed.
type Envelope struct {
	Event     EventType       `json:"event"`
	SessionID uuid.UUID       `json:"session_id"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
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
		Event:     event,
		SessionID: sessionID,
		Payload:   raw,
		Timestamp: time.Now().UTC(),
	}, nil
}
