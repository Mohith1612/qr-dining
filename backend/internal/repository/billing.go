package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Billing repository: org-level subscriptions, billing profiles, invoices, and
// manual payment records. Wrappers map pgx.ErrNoRows to domain errors and convert
// between clean Go types and the generated pgtype columns so services stay pgtype-free.

// ── Subscriptions ────────────────────────────────────────────────────────────

// CreateSubscriptionParams are the inputs for a new org subscription.
type CreateSubscriptionParams struct {
	OrganizationID int64
	PlanID         int64
	Status         string
	ProviderType   string
	StartedAt      *time.Time
	TrialEndsAt    *time.Time
	ExpiresAt      *time.Time
}

// UpdateSubscriptionParams is the full mutable field set written on every transition.
type UpdateSubscriptionParams struct {
	OrganizationID         int64
	PlanID                 int64
	Status                 string
	ProviderType           string
	ProviderSubscriptionID string
	ProviderCustomerID     string
	StartedAt              *time.Time
	TrialEndsAt            *time.Time
	ExpiresAt              *time.Time
	RenewedAt              *time.Time
	CancelledAt            *time.Time
	SuspendedAt            *time.Time
	CancellationReason     string
}

func (r *Repos) GetSubscriptionByOrg(ctx context.Context, orgID int64) (sqlc.OrganizationSubscription, error) {
	s, err := r.q.GetSubscriptionByOrg(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.OrganizationSubscription{}, domain.ErrSubscriptionNotFound
	}
	return s, err
}

func (r *Repos) CreateOrganizationSubscription(ctx context.Context, p CreateSubscriptionParams) (sqlc.OrganizationSubscription, error) {
	return r.q.CreateOrganizationSubscription(ctx, sqlc.CreateOrganizationSubscriptionParams{
		OrganizationID: p.OrganizationID,
		PlanID:         p.PlanID,
		Status:         p.Status,
		ProviderType:   p.ProviderType,
		StartedAt:      pgTimestamptz(p.StartedAt),
		TrialEndsAt:    pgTimestamptz(p.TrialEndsAt),
		ExpiresAt:      pgTimestamptz(p.ExpiresAt),
	})
}

func (r *Repos) UpdateOrganizationSubscription(ctx context.Context, p UpdateSubscriptionParams) (sqlc.OrganizationSubscription, error) {
	return r.q.UpdateOrganizationSubscription(ctx, sqlc.UpdateOrganizationSubscriptionParams{
		OrganizationID:         p.OrganizationID,
		PlanID:                 p.PlanID,
		Status:                 p.Status,
		ProviderType:           p.ProviderType,
		ProviderSubscriptionID: p.ProviderSubscriptionID,
		ProviderCustomerID:     p.ProviderCustomerID,
		StartedAt:              pgTimestamptz(p.StartedAt),
		TrialEndsAt:            pgTimestamptz(p.TrialEndsAt),
		ExpiresAt:              pgTimestamptz(p.ExpiresAt),
		RenewedAt:              pgTimestamptz(p.RenewedAt),
		CancelledAt:            pgTimestamptz(p.CancelledAt),
		SuspendedAt:            pgTimestamptz(p.SuspendedAt),
		CancellationReason:     p.CancellationReason,
	})
}

// ── Billing profile ──────────────────────────────────────────────────────────

// BillingProfileParams are the upsertable billing-profile fields.
type BillingProfileParams struct {
	OrganizationID int64
	BusinessName   string
	GstNumber      string
	TaxIdentifier  string
	BillingEmail   string
	BillingContact string
	BillingAddress string
	Currency       string
}

// GetBillingProfile returns the org's billing profile. pgx.ErrNoRows is passed
// through unmapped — "not set yet" is a normal state the service treats as empty.
func (r *Repos) GetBillingProfile(ctx context.Context, orgID int64) (sqlc.OrganizationBillingProfile, error) {
	return r.q.GetBillingProfile(ctx, orgID)
}

func (r *Repos) UpsertBillingProfile(ctx context.Context, p BillingProfileParams) (sqlc.OrganizationBillingProfile, error) {
	return r.q.UpsertBillingProfile(ctx, sqlc.UpsertBillingProfileParams{
		OrganizationID: p.OrganizationID,
		BusinessName:   p.BusinessName,
		GstNumber:      p.GstNumber,
		TaxIdentifier:  p.TaxIdentifier,
		BillingEmail:   p.BillingEmail,
		BillingContact: p.BillingContact,
		BillingAddress: p.BillingAddress,
		Currency:       p.Currency,
	})
}

// ── Invoices ─────────────────────────────────────────────────────────────────

// CreateInvoiceParams are the inputs for a new draft invoice.
type CreateInvoiceParams struct {
	OrganizationID          int64
	SubscriptionID          *int64
	InvoiceNumber           string
	Amount                  pgtype.Numeric
	Currency                string
	DueDate                 *time.Time
	Notes                   string
	CreatedByPlatformUserID *int64
}

func (r *Repos) NextInvoiceNumber(ctx context.Context) (int64, error) {
	return r.q.NextInvoiceNumber(ctx)
}

func (r *Repos) CreateInvoice(ctx context.Context, p CreateInvoiceParams) (sqlc.SubscriptionInvoice, error) {
	return r.q.CreateInvoice(ctx, sqlc.CreateInvoiceParams{
		OrganizationID:          p.OrganizationID,
		SubscriptionID:          pgInt8(p.SubscriptionID),
		InvoiceNumber:           p.InvoiceNumber,
		Column4:                 p.Amount,
		Currency:                p.Currency,
		DueDate:                 pgTimestamptz(p.DueDate),
		Notes:                   p.Notes,
		CreatedByPlatformUserID: pgInt8(p.CreatedByPlatformUserID),
	})
}

func (r *Repos) GetInvoiceByID(ctx context.Context, id int64) (sqlc.SubscriptionInvoice, error) {
	inv, err := r.q.GetInvoiceByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SubscriptionInvoice{}, domain.ErrInvoiceNotFound
	}
	return inv, err
}

func (r *Repos) ListInvoicesByOrg(ctx context.Context, orgID int64) ([]sqlc.SubscriptionInvoice, error) {
	return r.q.ListInvoicesByOrg(ctx, orgID)
}

func (r *Repos) UpdateInvoiceStatus(ctx context.Context, id int64, status string, issueDate, paidAt *time.Time) (sqlc.SubscriptionInvoice, error) {
	return r.q.UpdateInvoiceStatus(ctx, sqlc.UpdateInvoiceStatusParams{
		ID:        id,
		Status:    status,
		IssueDate: pgTimestamptz(issueDate),
		PaidAt:    pgTimestamptz(paidAt),
	})
}

// ── Manual payments ──────────────────────────────────────────────────────────

// CreatePaymentParams are the inputs for a manual payment record.
type CreatePaymentParams struct {
	OrganizationID           int64
	SubscriptionID           *int64
	InvoiceID                *int64
	ProviderType             string
	Method                   string
	Amount                   pgtype.Numeric
	Currency                 string
	ReferenceNumber          string
	Notes                    string
	ReceivedAt               time.Time
	RecordedByPlatformUserID *int64
}

func (r *Repos) CreateSubscriptionPayment(ctx context.Context, p CreatePaymentParams) (sqlc.SubscriptionPayment, error) {
	return r.q.CreateSubscriptionPayment(ctx, sqlc.CreateSubscriptionPaymentParams{
		OrganizationID:           p.OrganizationID,
		SubscriptionID:           pgInt8(p.SubscriptionID),
		InvoiceID:                pgInt8(p.InvoiceID),
		ProviderType:             p.ProviderType,
		Method:                   p.Method,
		Column6:                  p.Amount,
		Currency:                 p.Currency,
		ReferenceNumber:          p.ReferenceNumber,
		Notes:                    p.Notes,
		ReceivedAt:               p.ReceivedAt,
		RecordedByPlatformUserID: pgInt8(p.RecordedByPlatformUserID),
	})
}

func (r *Repos) ListPaymentsByOrg(ctx context.Context, orgID int64) ([]sqlc.SubscriptionPayment, error) {
	return r.q.ListPaymentsByOrg(ctx, orgID)
}

// ── pgtype conversion helpers ────────────────────────────────────────────────

func pgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func pgInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
