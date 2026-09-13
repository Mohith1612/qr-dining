package services

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
)

// Staff force-close — the recovery path for a table that left without paying or
// where something happened the app never recorded. It reuses closeSessionRecord
// (session.go) so it can never drift from the guest/host close; authorization
// lives with the caller, deliberately not here.

// ForceCloseResult reports what a staff force close actually did, so the audit
// entry can name the payments it disposed of.
//
// Session goes through credentialSafeSession before it is returned: the handler
// serializes this struct straight to the caller, and sqlc.Session carries
// session_token (F-8/F-27). The OpenAPI contract for this route already says so
// — it responds with #/components/schemas/Session, which has no such field.
type ForceCloseResult struct {
	Session sqlc.Session `json:"session"`
	// CancelledPaymentIDs are the non-terminal payments cancelled alongside the
	// session.
	CancelledPaymentIDs []int64 `json:"cancelled_payment_ids"`
	// StrandedPaymentIDs are non-terminal payments the domain transition table
	// refuses to move to cancelled. Reported, never force-written.
	StrandedPaymentIDs []int64 `json:"stranded_payment_ids,omitempty"`
}

// ForceCloseByStaff ends any session on the caller's own branch, for a table
// that left without paying or where something happened the app never recorded.
// Sessions otherwise end only by host action or by the age-based stale cleaner,
// which leaves staff no way to reclaim the table.
//
// It works regardless of payment state; see cancelOutstandingPayments for what
// happens to a payment still in flight. Authorization (role, branch) is the
// caller's responsibility — branchID here is the scope check, not the identity.
func (s *SessionService) ForceCloseByStaff(ctx context.Context, id uuid.UUID, staffID, branchID int64, reason string) (ForceCloseResult, error) {
	sess, err := s.repos.GetSessionByID(ctx, id)
	if err != nil {
		return ForceCloseResult{}, err
	}
	// Branch is derived from the session row, never from the caller.
	if sess.BranchID != branchID {
		return ForceCloseResult{}, domain.ErrSessionNotFound
	}
	if domain.IsSessionTerminal(domain.SessionStatus(sess.Status)) {
		return ForceCloseResult{}, domain.ErrSessionClosed
	}

	closed, err := s.closeSessionRecord(ctx, sess, sessionCloseActor{Type: "staff", ID: staffID})
	if err != nil {
		return ForceCloseResult{}, err
	}
	if !closed {
		// Raced another close between the read above and the update.
		return ForceCloseResult{}, domain.ErrSessionClosed
	}

	// Payments are disposed of after the close, not before: if this step fails
	// the table is at least reclaimed, and the leftover payment is still
	// reachable through the staff cancel route. The reverse order could leave a
	// session frozen in payment_pending with no cancellable payment left on it.
	cancelled, stranded, err := s.cancelOutstandingPayments(ctx, sess, staffID, reason)
	if err != nil {
		return ForceCloseResult{}, err
	}

	updated, err := s.repos.GetSessionByID(ctx, id)
	if err != nil {
		return ForceCloseResult{}, err
	}
	return ForceCloseResult{
		Session:             credentialSafeSession(updated),
		CancelledPaymentIDs: cancelled,
		StrandedPaymentIDs:  stranded,
	}, nil
}

// cancelOutstandingPayments moves every non-terminal payment on a force-closed
// session to cancelled. Leaving one behind strands it: the session is terminal,
// so no route would ever move it again, and it would sit in the staff pending
// payments queue and the stalled-payment escalation alert indefinitely.
//
// 'cancelled' is the honest terminal status — no money was collected, so
// 'completed' would fabricate revenue and 'failed' would blame a provider that
// never failed. No SESSION-scoped broadcast is emitted per payment: the session
// is already closed and SESSION_CLOSED supersedes it.
//
// A payment the domain transition table refuses to move (status 'pending') is
// reported as stranded rather than force-written — the state machine stays the
// authority.
func (s *SessionService) cancelOutstandingPayments(ctx context.Context, sess sqlc.Session, staffID int64, reason string) (cancelled, stranded []int64, err error) {
	payments, err := s.repos.ListPaymentsForSession(ctx, sess.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range payments {
		if isTerminalPaymentStatus(p.Status) {
			continue
		}
		if err := domain.ValidatePaymentTransition(domain.PaymentStatus(p.Status), domain.PaymentStatusCancelled); err != nil {
			stranded = append(stranded, p.ID)
			continue
		}
		if _, err := s.repos.UpdatePaymentStatusExpected(ctx, p.ID, p.Status, sqlc.PaymentStatusCancelled); err != nil {
			if errors.Is(err, domain.ErrInvalidPaymentTransition) {
				// Raced another writer; it is no longer in the status we read.
				stranded = append(stranded, p.ID)
				continue
			}
			return nil, nil, err
		}
		cancelled = append(cancelled, p.ID)
		s.repos.LogEvent(ctx, sess.ID, sess.BranchID, "PAYMENT_CANCELLED", "staff", staffID, map[string]any{
			"payment_id":      p.ID,
			"previous_status": string(p.Status),
			"reason":          reason,
			"cause":           "session_force_closed",
		})
	}
	return cancelled, stranded, nil
}
