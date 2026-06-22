package domain

// State machine tables define which transitions are valid for each entity.
// Services must call Validate*Transition before any status mutation.
// An empty allowed slice means the status is terminal — no transitions permitted.

// OrderStatus mirrors the PostgreSQL enum values.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusServed    OrderStatus = "served"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// SessionStatus mirrors the PostgreSQL enum values.
type SessionStatus string

const (
	SessionStatusActive               SessionStatus = "active"
	SessionStatusPaymentPending       SessionStatus = "payment_pending"
	SessionStatusAwaitingReactivation SessionStatus = "awaiting_reactivation"
	SessionStatusClosed               SessionStatus = "closed"
	SessionStatusAbandoned            SessionStatus = "abandoned"
	SessionStatusExpired              SessionStatus = "expired"
)

// IsSessionTerminal reports whether a session status accepts no further
// transitions. Reads against terminal sessions are bounded by the 60-minute
// read window in the snapshot handler.
func IsSessionTerminal(s SessionStatus) bool {
	switch s {
	case SessionStatusClosed, SessionStatusAbandoned, SessionStatusExpired:
		return true
	default:
		return false
	}
}

// IsSessionCartFrozen reports whether cart and order mutations must be rejected
// for a session in this status. Reads remain available.
func IsSessionCartFrozen(s SessionStatus) bool {
	return s == SessionStatusPaymentPending
}

// AssistanceStatus mirrors the PostgreSQL enum values.
type AssistanceStatus string

const (
	AssistanceStatusPending      AssistanceStatus = "pending"
	AssistanceStatusAcknowledged AssistanceStatus = "acknowledged"
	AssistanceStatusResolved     AssistanceStatus = "resolved"
)

// PaymentStatus mirrors the PostgreSQL enum values.
type PaymentStatus string

const (
	PaymentStatusPending                   PaymentStatus = "pending"
	PaymentStatusRequested                 PaymentStatus = "requested"
	PaymentStatusProviderPending           PaymentStatus = "provider_pending"
	PaymentStatusRequiresStaffConfirmation PaymentStatus = "requires_staff_confirmation"
	PaymentStatusCompleted                 PaymentStatus = "completed"
	PaymentStatusFailed                    PaymentStatus = "failed"
	PaymentStatusCancelled                 PaymentStatus = "cancelled"
	PaymentStatusRefunded                  PaymentStatus = "refunded"
	PaymentStatusPartiallyRefunded         PaymentStatus = "partially_refunded"
)

var orderTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPending:   {OrderStatusConfirmed, OrderStatusCancelled},
	OrderStatusConfirmed: {OrderStatusPreparing, OrderStatusCancelled},
	OrderStatusPreparing: {OrderStatusReady},
	OrderStatusReady:     {OrderStatusServed},
	OrderStatusServed:    {},
	OrderStatusCancelled: {},
}

var sessionTransitions = map[SessionStatus][]SessionStatus{
	SessionStatusActive: {
		SessionStatusPaymentPending,
		SessionStatusAwaitingReactivation,
		SessionStatusClosed,
		SessionStatusAbandoned,
		SessionStatusExpired,
	},
	SessionStatusPaymentPending: {
		SessionStatusActive,
		SessionStatusAwaitingReactivation,
		SessionStatusClosed,
		SessionStatusAbandoned,
		SessionStatusExpired,
	},
	SessionStatusAwaitingReactivation: {
		SessionStatusActive,
		SessionStatusPaymentPending,
		SessionStatusAbandoned,
		SessionStatusExpired,
		SessionStatusClosed,
	},
	SessionStatusClosed:    {},
	SessionStatusAbandoned: {},
	SessionStatusExpired:   {},
}

var assistanceTransitions = map[AssistanceStatus][]AssistanceStatus{
	AssistanceStatusPending:      {AssistanceStatusAcknowledged, AssistanceStatusResolved},
	AssistanceStatusAcknowledged: {AssistanceStatusResolved},
	AssistanceStatusResolved:     {},
}

var paymentTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusPending:                   {PaymentStatusCompleted, PaymentStatusFailed, PaymentStatusProviderPending, PaymentStatusRequiresStaffConfirmation},
	PaymentStatusRequested:                 {PaymentStatusProviderPending, PaymentStatusRequiresStaffConfirmation, PaymentStatusCancelled, PaymentStatusFailed},
	PaymentStatusProviderPending:           {PaymentStatusCompleted, PaymentStatusFailed, PaymentStatusCancelled},
	PaymentStatusRequiresStaffConfirmation: {PaymentStatusCompleted, PaymentStatusFailed, PaymentStatusCancelled},
	PaymentStatusCompleted:                 {PaymentStatusRefunded, PaymentStatusPartiallyRefunded},
	PaymentStatusPartiallyRefunded:         {PaymentStatusRefunded},
	PaymentStatusFailed:                    {PaymentStatusPending, PaymentStatusRequested},
	PaymentStatusCancelled:                 {},
	PaymentStatusRefunded:                  {},
}

func ValidateOrderTransition(from, to OrderStatus) error {
	return validateTransition(from, to, orderTransitions, ErrInvalidOrderTransition)
}

func ValidateSessionTransition(from, to SessionStatus) error {
	return validateTransition(from, to, sessionTransitions, ErrSessionClosed)
}

func ValidateAssistanceTransition(from, to AssistanceStatus) error {
	return validateTransition(from, to, assistanceTransitions, ErrInvalidAssistanceTransition)
}

func ValidatePaymentTransition(from, to PaymentStatus) error {
	return validateTransition(from, to, paymentTransitions, ErrInvalidPaymentTransition)
}

func validateTransition[T comparable](from, to T, table map[T][]T, sentinel error) error {
	allowed, ok := table[from]
	if !ok {
		return sentinel
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return sentinel
}
