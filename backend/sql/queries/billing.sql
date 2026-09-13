-- Org-level subscription lifecycle + billing foundation queries.
-- Mutations are driven by BillingService; status is recorded/audited, not enforced.

-- name: GetSubscriptionByOrg :one
SELECT * FROM organization_subscriptions WHERE organization_id = $1;

-- name: CreateOrganizationSubscription :one
INSERT INTO organization_subscriptions (
    organization_id, plan_id, status, provider_type,
    started_at, trial_ends_at, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateOrganizationSubscription :one
UPDATE organization_subscriptions
SET plan_id                  = $2,
    status                   = $3,
    provider_type            = $4,
    provider_subscription_id = $5,
    provider_customer_id     = $6,
    started_at               = $7,
    trial_ends_at            = $8,
    expires_at               = $9,
    renewed_at               = $10,
    cancelled_at             = $11,
    suspended_at             = $12,
    cancellation_reason      = $13,
    updated_at               = NOW()
WHERE organization_id = $1
RETURNING *;

-- name: GetBillingProfile :one
SELECT * FROM organization_billing_profiles WHERE organization_id = $1;

-- name: UpsertBillingProfile :one
INSERT INTO organization_billing_profiles (
    organization_id, business_name, gst_number, tax_identifier,
    billing_email, billing_contact, billing_address, currency
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (organization_id) DO UPDATE SET
    business_name   = EXCLUDED.business_name,
    gst_number      = EXCLUDED.gst_number,
    tax_identifier  = EXCLUDED.tax_identifier,
    billing_email   = EXCLUDED.billing_email,
    billing_contact = EXCLUDED.billing_contact,
    billing_address = EXCLUDED.billing_address,
    currency        = EXCLUDED.currency,
    updated_at      = NOW()
RETURNING *;

-- name: NextInvoiceNumber :one
SELECT nextval('billing_invoice_number_seq')::bigint AS seq;

-- name: CreateInvoice :one
INSERT INTO subscription_invoices (
    organization_id, subscription_id, invoice_number, status,
    amount, currency, due_date, notes, created_by_platform_user_id
) VALUES ($1, $2, $3, 'draft', $4::numeric, $5, $6, $7, $8)
RETURNING *;

-- name: GetInvoiceByID :one
SELECT * FROM subscription_invoices WHERE id = $1;

-- name: ListInvoicesByOrg :many
SELECT * FROM subscription_invoices WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: UpdateInvoiceStatus :one
UPDATE subscription_invoices
SET status     = $2,
    issue_date = $3,
    paid_at    = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateSubscriptionPayment :one
INSERT INTO subscription_payments (
    organization_id, subscription_id, invoice_id, provider_type, method,
    amount, currency, reference_number, notes, received_at, recorded_by_platform_user_id
) VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListPaymentsByOrg :many
SELECT * FROM subscription_payments WHERE organization_id = $1 ORDER BY received_at DESC;
