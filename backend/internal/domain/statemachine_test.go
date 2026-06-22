package domain_test

import (
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
)

func TestValidateOrderTransition(t *testing.T) {
	tests := []struct {
		from    domain.OrderStatus
		to      domain.OrderStatus
		wantErr bool
	}{
		{domain.OrderStatusPending, domain.OrderStatusConfirmed, false},
		{domain.OrderStatusPending, domain.OrderStatusCancelled, false},
		{domain.OrderStatusConfirmed, domain.OrderStatusPreparing, false},
		{domain.OrderStatusConfirmed, domain.OrderStatusCancelled, false},
		{domain.OrderStatusPreparing, domain.OrderStatusReady, false},
		{domain.OrderStatusReady, domain.OrderStatusServed, false},
		// Invalid transitions
		{domain.OrderStatusPending, domain.OrderStatusServed, true},
		{domain.OrderStatusServed, domain.OrderStatusPending, true},
		{domain.OrderStatusCancelled, domain.OrderStatusConfirmed, true},
		{domain.OrderStatusServed, domain.OrderStatusCancelled, true},
	}

	for _, tt := range tests {
		err := domain.ValidateOrderTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateOrderTransition(%s→%s): got err=%v, wantErr=%v", tt.from, tt.to, err, tt.wantErr)
		}
	}
}

func TestValidateSessionTransition(t *testing.T) {
	tests := []struct {
		from    domain.SessionStatus
		to      domain.SessionStatus
		wantErr bool
	}{
		// Original transitions
		{domain.SessionStatusActive, domain.SessionStatusClosed, false},
		{domain.SessionStatusActive, domain.SessionStatusAbandoned, false},
		// Phase A intermediate states
		{domain.SessionStatusActive, domain.SessionStatusPaymentPending, false},
		{domain.SessionStatusPaymentPending, domain.SessionStatusActive, false},
		{domain.SessionStatusPaymentPending, domain.SessionStatusClosed, false},
		{domain.SessionStatusActive, domain.SessionStatusAwaitingReactivation, false},
		{domain.SessionStatusAwaitingReactivation, domain.SessionStatusActive, false},
		{domain.SessionStatusAwaitingReactivation, domain.SessionStatusAbandoned, false},
		{domain.SessionStatusActive, domain.SessionStatusExpired, false},
		// Terminal states
		{domain.SessionStatusClosed, domain.SessionStatusActive, true},
		{domain.SessionStatusAbandoned, domain.SessionStatusActive, true},
		{domain.SessionStatusClosed, domain.SessionStatusAbandoned, true},
		{domain.SessionStatusExpired, domain.SessionStatusActive, true},
		{domain.SessionStatusExpired, domain.SessionStatusClosed, true},
	}

	for _, tt := range tests {
		err := domain.ValidateSessionTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSessionTransition(%s→%s): got err=%v, wantErr=%v", tt.from, tt.to, err, tt.wantErr)
		}
	}
}

func TestIsSessionTerminal(t *testing.T) {
	terminal := []domain.SessionStatus{domain.SessionStatusClosed, domain.SessionStatusAbandoned, domain.SessionStatusExpired}
	for _, s := range terminal {
		if !domain.IsSessionTerminal(s) {
			t.Errorf("IsSessionTerminal(%s) = false, want true", s)
		}
	}
	nonTerminal := []domain.SessionStatus{domain.SessionStatusActive, domain.SessionStatusPaymentPending, domain.SessionStatusAwaitingReactivation}
	for _, s := range nonTerminal {
		if domain.IsSessionTerminal(s) {
			t.Errorf("IsSessionTerminal(%s) = true, want false", s)
		}
	}
}

func TestIsSessionCartFrozen(t *testing.T) {
	if !domain.IsSessionCartFrozen(domain.SessionStatusPaymentPending) {
		t.Error("payment_pending should freeze cart")
	}
	for _, s := range []domain.SessionStatus{
		domain.SessionStatusActive,
		domain.SessionStatusAwaitingReactivation,
		domain.SessionStatusClosed,
		domain.SessionStatusAbandoned,
		domain.SessionStatusExpired,
	} {
		if domain.IsSessionCartFrozen(s) {
			t.Errorf("IsSessionCartFrozen(%s) = true, want false", s)
		}
	}
}

func TestValidateAssistanceTransition(t *testing.T) {
	tests := []struct {
		from    domain.AssistanceStatus
		to      domain.AssistanceStatus
		wantErr bool
	}{
		{domain.AssistanceStatusPending, domain.AssistanceStatusAcknowledged, false},
		{domain.AssistanceStatusPending, domain.AssistanceStatusResolved, false},
		{domain.AssistanceStatusAcknowledged, domain.AssistanceStatusResolved, false},
		// Invalid
		{domain.AssistanceStatusResolved, domain.AssistanceStatusPending, true},
		{domain.AssistanceStatusResolved, domain.AssistanceStatusAcknowledged, true},
		{domain.AssistanceStatusAcknowledged, domain.AssistanceStatusPending, true},
	}

	for _, tt := range tests {
		err := domain.ValidateAssistanceTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateAssistanceTransition(%s→%s): got err=%v, wantErr=%v", tt.from, tt.to, err, tt.wantErr)
		}
	}
}

func TestValidatePaymentTransition(t *testing.T) {
	tests := []struct {
		from    domain.PaymentStatus
		to      domain.PaymentStatus
		wantErr bool
	}{
		{domain.PaymentStatusPending, domain.PaymentStatusCompleted, false},
		{domain.PaymentStatusPending, domain.PaymentStatusFailed, false},
		{domain.PaymentStatusCompleted, domain.PaymentStatusRefunded, false},
		{domain.PaymentStatusFailed, domain.PaymentStatusPending, false},
		// Invalid
		{domain.PaymentStatusRefunded, domain.PaymentStatusPending, true},
		{domain.PaymentStatusCompleted, domain.PaymentStatusPending, true},
		{domain.PaymentStatusCompleted, domain.PaymentStatusFailed, true},
	}

	for _, tt := range tests {
		err := domain.ValidatePaymentTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePaymentTransition(%s→%s): got err=%v, wantErr=%v", tt.from, tt.to, err, tt.wantErr)
		}
	}
}
