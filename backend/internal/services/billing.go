package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// BillingService owns the org-level subscription lifecycle, billing profile,
// manual payment records, and invoices. It is the authoritative source for
// billing state.
//
// Resolve -> observe -> enforce: subscription STATUS is recorded and audited but
// is NOT enforced on any operational path in this phase. The subscription's PLAN is
// kept in sync with organization_plan_assignments (which the entitlement resolver
// reads) so entitlements track the plan; the resolver itself is unchanged.
type BillingService struct {
	repos *repository.Repos
}

func NewBillingService(repos *repository.Repos) *BillingService {
	return &BillingService{repos: repos}
}

// ── valid status sets / transitions ──────────────────────────────────────────

const (
	SubStatusTrial     = "trial"
	SubStatusActive    = "active"
	SubStatusSuspended = "suspended"
	SubStatusCancelled = "cancelled"
	SubStatusExpired   = "expired"
	SubStatusPastDue   = "past_due"
)

// subscriptionTransitions maps an action to the set of statuses it may apply from.
// activate and extend_trial may also create a subscription when none exists.
var subscriptionTransitions = map[string]map[string]bool{
	"activate":     {SubStatusTrial: true, SubStatusActive: true, SubStatusSuspended: true, SubStatusPastDue: true, SubStatusExpired: true, SubStatusCancelled: true},
	"suspend":      {SubStatusTrial: true, SubStatusActive: true, SubStatusPastDue: true},
	"renew":        {SubStatusActive: true, SubStatusPastDue: true, SubStatusExpired: true},
	"cancel":       {SubStatusTrial: true, SubStatusActive: true, SubStatusSuspended: true, SubStatusPastDue: true, SubStatusExpired: true},
	"extend_trial": {SubStatusTrial: true},
	"change_plan":  {SubStatusTrial: true, SubStatusActive: true, SubStatusSuspended: true, SubStatusPastDue: true, SubStatusExpired: true},
}

// validSubscriptionTransition reports whether action is permitted from the given status.
func validSubscriptionTransition(action, from string) bool {
	return subscriptionTransitions[action][from]
}

// mappedAssignmentStatus maps the rich subscription status onto the coarser
// organization_plan_assignments status (CHECK: trial|active|suspended|cancelled).
func mappedAssignmentStatus(subStatus string) string {
	switch subStatus {
	case SubStatusTrial:
		return "trial"
	case SubStatusActive, SubStatusPastDue:
		return "active"
	case SubStatusSuspended:
		return "suspended"
	default: // cancelled, expired
		return "cancelled"
	}
}

var validPaymentMethods = map[string]bool{
	"upi": true, "bank_transfer": true, "cash": true, "cheque": true, "other": true,
}

// ── subscription reads ───────────────────────────────────────────────────────

// GetSubscription returns the org's subscription (domain.ErrSubscriptionNotFound if none).
func (s *BillingService) GetSubscription(ctx context.Context, orgID int64) (sqlc.OrganizationSubscription, error) {
	return s.repos.GetSubscriptionByOrg(ctx, orgID)
}

// ── subscription transitions ─────────────────────────────────────────────────

// Activate creates (if absent) or transitions a subscription to active. planID is
// required when creating; when transitioning an existing subscription it is optional
// (nil keeps the current plan). expiresAt sets the paid-period end.
func (s *BillingService) Activate(ctx context.Context, orgID int64, planID *int64, expiresAt *time.Time, actorID int64) (sqlc.OrganizationSubscription, error) {
	now := time.Now().UTC()
	var out sqlc.OrganizationSubscription
	err := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		cur, err := tx.GetSubscriptionByOrg(ctx, orgID)
		switch {
		case errors.Is(err, domain.ErrSubscriptionNotFound):
			if planID == nil {
				return errPlanRequired
			}
			created, cerr := tx.CreateOrganizationSubscription(ctx, repository.CreateSubscriptionParams{
				OrganizationID: orgID,
				PlanID:         *planID,
				Status:         SubStatusActive,
				ProviderType:   "manual",
				StartedAt:      &now,
				ExpiresAt:      expiresAt,
			})
			if cerr != nil {
				return cerr
			}
			out = created
		case err != nil:
			return err
		default:
			if !validSubscriptionTransition("activate", cur.Status) {
				return domain.ErrInvalidSubscriptionTransition
			}
			next := mutateForActivate(cur, planID, expiresAt, now)
			updated, uerr := tx.UpdateOrganizationSubscription(ctx, next)
			if uerr != nil {
				return uerr
			}
			out = updated
		}
		return syncPlanAssignment(ctx, tx, orgID, out.PlanID, out.Status, actorID)
	})
	return out, err
}

// ExtendTrial creates (if absent) or extends a trial subscription. planID is required
// when creating. trialEndsAt is the new trial end.
func (s *BillingService) ExtendTrial(ctx context.Context, orgID int64, planID *int64, trialEndsAt time.Time, actorID int64) (sqlc.OrganizationSubscription, error) {
	now := time.Now().UTC()
	var out sqlc.OrganizationSubscription
	err := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		cur, err := tx.GetSubscriptionByOrg(ctx, orgID)
		switch {
		case errors.Is(err, domain.ErrSubscriptionNotFound):
			if planID == nil {
				return errPlanRequired
			}
			created, cerr := tx.CreateOrganizationSubscription(ctx, repository.CreateSubscriptionParams{
				OrganizationID: orgID,
				PlanID:         *planID,
				Status:         SubStatusTrial,
				ProviderType:   "manual",
				StartedAt:      &now,
				TrialEndsAt:    &trialEndsAt,
			})
			if cerr != nil {
				return cerr
			}
			out = created
		case err != nil:
			return err
		default:
			if !validSubscriptionTransition("extend_trial", cur.Status) {
				return domain.ErrInvalidSubscriptionTransition
			}
			next := baseUpdate(cur)
			if planID != nil {
				next.PlanID = *planID
			}
			next.TrialEndsAt = &trialEndsAt
			updated, uerr := tx.UpdateOrganizationSubscription(ctx, next)
			if uerr != nil {
				return uerr
			}
			out = updated
		}
		return syncPlanAssignment(ctx, tx, orgID, out.PlanID, out.Status, actorID)
	})
	return out, err
}

// Suspend transitions an existing subscription to suspended.
func (s *BillingService) Suspend(ctx context.Context, orgID, actorID int64) (sqlc.OrganizationSubscription, error) {
	return s.transition(ctx, orgID, "suspend", actorID, func(next *repository.UpdateSubscriptionParams, now time.Time) {
		next.Status = SubStatusSuspended
		next.SuspendedAt = &now
	})
}

// Renew transitions an existing subscription to active and extends expiry.
func (s *BillingService) Renew(ctx context.Context, orgID int64, expiresAt *time.Time, actorID int64) (sqlc.OrganizationSubscription, error) {
	return s.transition(ctx, orgID, "renew", actorID, func(next *repository.UpdateSubscriptionParams, now time.Time) {
		next.Status = SubStatusActive
		next.RenewedAt = &now
		if next.StartedAt == nil {
			next.StartedAt = &now
		}
		if expiresAt != nil {
			next.ExpiresAt = expiresAt
		}
		next.SuspendedAt = nil
	})
}

// Cancel transitions an existing subscription to cancelled with a reason.
func (s *BillingService) Cancel(ctx context.Context, orgID int64, reason string, actorID int64) (sqlc.OrganizationSubscription, error) {
	return s.transition(ctx, orgID, "cancel", actorID, func(next *repository.UpdateSubscriptionParams, now time.Time) {
		next.Status = SubStatusCancelled
		next.CancelledAt = &now
		next.CancellationReason = strings.TrimSpace(reason)
	})
}

// ChangePlan upgrades/downgrades the plan on an existing subscription (status unchanged).
func (s *BillingService) ChangePlan(ctx context.Context, orgID, planID, actorID int64) (sqlc.OrganizationSubscription, error) {
	return s.transition(ctx, orgID, "change_plan", actorID, func(next *repository.UpdateSubscriptionParams, _ time.Time) {
		next.PlanID = planID
	})
}

// transition is the shared read-validate-mutate-write-sync flow for actions that
// require an existing subscription.
func (s *BillingService) transition(ctx context.Context, orgID int64, action string, actorID int64, mutate func(*repository.UpdateSubscriptionParams, time.Time)) (sqlc.OrganizationSubscription, error) {
	now := time.Now().UTC()
	var out sqlc.OrganizationSubscription
	err := s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		cur, err := tx.GetSubscriptionByOrg(ctx, orgID)
		if err != nil {
			return err
		}
		if !validSubscriptionTransition(action, cur.Status) {
			return domain.ErrInvalidSubscriptionTransition
		}
		next := baseUpdate(cur)
		mutate(&next, now)
		updated, uerr := tx.UpdateOrganizationSubscription(ctx, next)
		if uerr != nil {
			return uerr
		}
		out = updated
		return syncPlanAssignment(ctx, tx, orgID, out.PlanID, out.Status, actorID)
	})
	return out, err
}

// syncPlanAssignment keeps organization_plan_assignments aligned with the subscription
// so the entitlement resolver tracks the subscription's plan. Status is mapped to the
// assignment's coarser CHECK set; the resolver reads the plan, not the status.
func syncPlanAssignment(ctx context.Context, tx *repository.Repos, orgID, planID int64, subStatus string, actorID int64) error {
	var actor *int64
	if actorID != 0 {
		a := actorID
		actor = &a
	}
	_, err := tx.UpsertOrganizationPlanAssignment(ctx, orgID, planID, mappedAssignmentStatus(subStatus), actor)
	return err
}

// baseUpdate seeds an UpdateSubscriptionParams from the current row so unchanged
// fields are preserved across a transition.
func baseUpdate(cur sqlc.OrganizationSubscription) repository.UpdateSubscriptionParams {
	return repository.UpdateSubscriptionParams{
		OrganizationID:         cur.OrganizationID,
		PlanID:                 cur.PlanID,
		Status:                 cur.Status,
		ProviderType:           cur.ProviderType,
		ProviderSubscriptionID: cur.ProviderSubscriptionID,
		ProviderCustomerID:     cur.ProviderCustomerID,
		StartedAt:              timePtr(cur.StartedAt),
		TrialEndsAt:            timePtr(cur.TrialEndsAt),
		ExpiresAt:              timePtr(cur.ExpiresAt),
		RenewedAt:              timePtr(cur.RenewedAt),
		CancelledAt:            timePtr(cur.CancelledAt),
		SuspendedAt:            timePtr(cur.SuspendedAt),
		CancellationReason:     cur.CancellationReason,
	}
}

// mutateForActivate applies the activate transition to an existing subscription.
func mutateForActivate(cur sqlc.OrganizationSubscription, planID *int64, expiresAt *time.Time, now time.Time) repository.UpdateSubscriptionParams {
	next := baseUpdate(cur)
	next.Status = SubStatusActive
	if planID != nil {
		next.PlanID = *planID
	}
	if next.StartedAt == nil {
		next.StartedAt = &now
	}
	next.RenewedAt = &now
	if expiresAt != nil {
		next.ExpiresAt = expiresAt
	}
	// Activation clears prior suspension/cancellation.
	next.SuspendedAt = nil
	next.CancelledAt = nil
	next.CancellationReason = ""
	return next
}

// ── billing profile ──────────────────────────────────────────────────────────

// GetBillingProfile returns the org's billing profile and whether it has been set.
func (s *BillingService) GetBillingProfile(ctx context.Context, orgID int64) (sqlc.OrganizationBillingProfile, bool, error) {
	p, err := s.repos.GetBillingProfile(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.OrganizationBillingProfile{OrganizationID: orgID, Currency: "INR"}, false, nil
	}
	if err != nil {
		return sqlc.OrganizationBillingProfile{}, false, err
	}
	return p, true, nil
}

// UpsertBillingProfile creates or updates the org's billing metadata.
func (s *BillingService) UpsertBillingProfile(ctx context.Context, p repository.BillingProfileParams) (sqlc.OrganizationBillingProfile, error) {
	if strings.TrimSpace(p.Currency) == "" {
		p.Currency = "INR"
	}
	return s.repos.UpsertBillingProfile(ctx, p)
}

// ── invoices ─────────────────────────────────────────────────────────────────

var invoiceTransitions = map[string]map[string]bool{
	"issue":     {"draft": true},
	"mark_paid": {"draft": true, "issued": true},
	"cancel":    {"draft": true, "issued": true},
}

func validInvoiceTransition(action, from string) bool {
	return invoiceTransitions[action][from]
}

// formatInvoiceNumber renders a human-readable, unique invoice number from the sequence.
func formatInvoiceNumber(year, seq int64) string {
	return fmt.Sprintf("INV-%d-%06d", year, seq)
}

// CreateInvoice creates a draft invoice. amount is a decimal string. It links the
// org's current subscription when one exists.
func (s *BillingService) CreateInvoice(ctx context.Context, orgID int64, amount, currency string, dueDate *time.Time, notes string, actorID int64) (sqlc.SubscriptionInvoice, error) {
	amt, err := parseNumeric(amount)
	if err != nil {
		return sqlc.SubscriptionInvoice{}, err
	}
	if strings.TrimSpace(currency) == "" {
		currency = "INR"
	}
	var subID *int64
	if sub, serr := s.repos.GetSubscriptionByOrg(ctx, orgID); serr == nil {
		subID = &sub.ID
	} else if !errors.Is(serr, domain.ErrSubscriptionNotFound) {
		return sqlc.SubscriptionInvoice{}, serr
	}
	seq, err := s.repos.NextInvoiceNumber(ctx)
	if err != nil {
		return sqlc.SubscriptionInvoice{}, err
	}
	var actor *int64
	if actorID != 0 {
		actor = &actorID
	}
	return s.repos.CreateInvoice(ctx, repository.CreateInvoiceParams{
		OrganizationID:          orgID,
		SubscriptionID:          subID,
		InvoiceNumber:           formatInvoiceNumber(int64(time.Now().UTC().Year()), seq),
		Amount:                  amt,
		Currency:                currency,
		DueDate:                 dueDate,
		Notes:                   strings.TrimSpace(notes),
		CreatedByPlatformUserID: actor,
	})
}

// GetInvoice returns an invoice, enforcing that it belongs to orgID (tenant isolation).
func (s *BillingService) GetInvoice(ctx context.Context, orgID, invoiceID int64) (sqlc.SubscriptionInvoice, error) {
	inv, err := s.repos.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return sqlc.SubscriptionInvoice{}, err
	}
	if inv.OrganizationID != orgID {
		return sqlc.SubscriptionInvoice{}, domain.ErrInvoiceNotFound
	}
	return inv, nil
}

// ListInvoices returns the org's invoices, newest first.
func (s *BillingService) ListInvoices(ctx context.Context, orgID int64) ([]sqlc.SubscriptionInvoice, error) {
	return s.repos.ListInvoicesByOrg(ctx, orgID)
}

// TransitionInvoice applies an invoice lifecycle action (issue|mark_paid|cancel),
// enforcing org ownership and valid transitions.
func (s *BillingService) TransitionInvoice(ctx context.Context, orgID, invoiceID int64, action string) (sqlc.SubscriptionInvoice, error) {
	inv, err := s.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return sqlc.SubscriptionInvoice{}, err
	}
	if !validInvoiceTransition(action, inv.Status) {
		return sqlc.SubscriptionInvoice{}, domain.ErrInvalidInvoiceTransition
	}
	now := time.Now().UTC()
	status := inv.Status
	issueDate := timePtr(inv.IssueDate)
	paidAt := timePtr(inv.PaidAt)
	switch action {
	case "issue":
		status = "issued"
		issueDate = &now
	case "mark_paid":
		status = "paid"
		if issueDate == nil {
			issueDate = &now
		}
		paidAt = &now
	case "cancel":
		status = "cancelled"
	}
	return s.repos.UpdateInvoiceStatus(ctx, invoiceID, status, issueDate, paidAt)
}

// ── manual payments ──────────────────────────────────────────────────────────

// RecordPayment records a manual payment for bookkeeping. method must be a known
// manual method; amount is a decimal string. An optional invoiceID is validated to
// belong to the same org.
func (s *BillingService) RecordPayment(ctx context.Context, orgID int64, method, amount, currency, reference, notes string, receivedAt *time.Time, invoiceID *int64, actorID int64) (sqlc.SubscriptionPayment, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	if !validPaymentMethods[method] {
		return sqlc.SubscriptionPayment{}, errInvalidPaymentMethod
	}
	amt, err := parseNumeric(amount)
	if err != nil {
		return sqlc.SubscriptionPayment{}, err
	}
	if strings.TrimSpace(currency) == "" {
		currency = "INR"
	}
	if invoiceID != nil {
		if _, ierr := s.GetInvoice(ctx, orgID, *invoiceID); ierr != nil {
			return sqlc.SubscriptionPayment{}, ierr
		}
	}
	var subID *int64
	if sub, serr := s.repos.GetSubscriptionByOrg(ctx, orgID); serr == nil {
		subID = &sub.ID
	}
	when := time.Now().UTC()
	if receivedAt != nil {
		when = *receivedAt
	}
	var actor *int64
	if actorID != 0 {
		actor = &actorID
	}
	return s.repos.CreateSubscriptionPayment(ctx, repository.CreatePaymentParams{
		OrganizationID:           orgID,
		SubscriptionID:           subID,
		InvoiceID:                invoiceID,
		ProviderType:             "manual",
		Method:                   method,
		Amount:                   amt,
		Currency:                 currency,
		ReferenceNumber:          strings.TrimSpace(reference),
		Notes:                    strings.TrimSpace(notes),
		ReceivedAt:               when,
		RecordedByPlatformUserID: actor,
	})
}

// ListPayments returns the org's manual payment records, newest first.
func (s *BillingService) ListPayments(ctx context.Context, orgID int64) ([]sqlc.SubscriptionPayment, error) {
	return s.repos.ListPaymentsByOrg(ctx, orgID)
}

// ── helpers / errors ─────────────────────────────────────────────────────────

var (
	errPlanRequired         = errors.New("plan_id is required to create a subscription")
	errInvalidPaymentMethod = errors.New("invalid payment method")
)

// ErrPlanRequired / ErrInvalidPaymentMethod are exported predicates for handler mapping.
func IsPlanRequired(err error) bool         { return errors.Is(err, errPlanRequired) }
func IsInvalidPaymentMethod(err error) bool { return errors.Is(err, errInvalidPaymentMethod) }

// parseNumeric converts a decimal string into a pgtype.Numeric and rejects negatives.
func parseNumeric(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strings.TrimSpace(s)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("invalid amount: %w", err)
	}
	return n, nil
}

// timePtr converts a nullable pgtype.Timestamptz to *time.Time.
func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
