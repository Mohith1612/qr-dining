-- Platform billing foundation: org-level subscription lifecycle, billing metadata,
-- manual payment bookkeeping, and invoices. Additive only.
--
-- Resolve-only/shadow: subscription STATUS is recorded + audited but NOT enforced on
-- any operational path in this phase (resolve -> observe -> enforce). The platform
-- plan-change/activate actions sync organization_plan_assignments so the entitlement
-- resolver tracks the subscription's PLAN; the resolver itself is unchanged.
--
-- Future-gateway compatible: a future Razorpay/Stripe integration is a new provider_type
-- feeding these same tables (provider_* columns + metadata_json absorb external refs/
-- payloads) — no schema/subscription/entitlement/plan redesign required.

-- Org-level subscription (one per organization). Authoritative for lifecycle + billing.
CREATE TABLE organization_subscriptions (
    id                       BIGSERIAL   PRIMARY KEY,
    organization_id          BIGINT      NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id                  BIGINT      NOT NULL REFERENCES subscription_plans(id),
    status                   TEXT        NOT NULL DEFAULT 'trial'
                                         CHECK (status IN ('trial', 'active', 'suspended', 'cancelled', 'expired', 'past_due')),
    provider_type            TEXT        NOT NULL DEFAULT 'manual'
                                         CHECK (provider_type IN ('manual', 'razorpay', 'stripe')),
    provider_subscription_id TEXT        NOT NULL DEFAULT '',
    provider_customer_id     TEXT        NOT NULL DEFAULT '',
    started_at               TIMESTAMPTZ,
    trial_ends_at            TIMESTAMPTZ,
    expires_at               TIMESTAMPTZ,
    renewed_at               TIMESTAMPTZ,
    cancelled_at             TIMESTAMPTZ,
    suspended_at             TIMESTAMPTZ,
    cancellation_reason      TEXT        NOT NULL DEFAULT '',
    metadata_json            JSONB       NOT NULL DEFAULT '{}',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organization_subscriptions_status ON organization_subscriptions(status);

-- Org billing metadata (one per organization). No payment processing.
CREATE TABLE organization_billing_profiles (
    organization_id BIGINT      PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    business_name   TEXT        NOT NULL DEFAULT '',
    gst_number      TEXT        NOT NULL DEFAULT '',
    tax_identifier  TEXT        NOT NULL DEFAULT '',
    billing_email   TEXT        NOT NULL DEFAULT '',
    billing_contact TEXT        NOT NULL DEFAULT '',
    billing_address TEXT        NOT NULL DEFAULT '',
    currency        TEXT        NOT NULL DEFAULT 'INR',
    metadata_json   JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Invoice foundation. No PDF generation. invoice_number formatted in-app from this sequence.
CREATE SEQUENCE billing_invoice_number_seq;

CREATE TABLE subscription_invoices (
    id                          BIGSERIAL     PRIMARY KEY,
    organization_id             BIGINT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subscription_id             BIGINT        REFERENCES organization_subscriptions(id) ON DELETE SET NULL,
    invoice_number              TEXT          NOT NULL UNIQUE,
    status                      TEXT          NOT NULL DEFAULT 'draft'
                                              CHECK (status IN ('draft', 'issued', 'paid', 'cancelled')),
    amount                      NUMERIC(12, 2) NOT NULL DEFAULT 0,
    currency                    TEXT          NOT NULL DEFAULT 'INR',
    issue_date                  TIMESTAMPTZ,
    due_date                    TIMESTAMPTZ,
    paid_at                     TIMESTAMPTZ,
    notes                       TEXT          NOT NULL DEFAULT '',
    metadata_json               JSONB         NOT NULL DEFAULT '{}',
    created_by_platform_user_id BIGINT        REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscription_invoices_organization ON subscription_invoices(organization_id, status);

-- Manual payment records (bookkeeping). Generic payment-provider abstraction:
-- provider_type=manual today; future gateways set provider_type + provider_payment_id.
-- These are INTERNAL records, NOT customer-facing checkout payments.
CREATE TABLE subscription_payments (
    id                           BIGSERIAL     PRIMARY KEY,
    organization_id              BIGINT        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subscription_id              BIGINT        REFERENCES organization_subscriptions(id) ON DELETE SET NULL,
    invoice_id                   BIGINT        REFERENCES subscription_invoices(id) ON DELETE SET NULL,
    provider_type                TEXT          NOT NULL DEFAULT 'manual'
                                               CHECK (provider_type IN ('manual', 'razorpay', 'stripe')),
    method                       TEXT          NOT NULL
                                               CHECK (method IN ('upi', 'bank_transfer', 'cash', 'cheque', 'other')),
    amount                       NUMERIC(12, 2) NOT NULL,
    currency                     TEXT          NOT NULL DEFAULT 'INR',
    reference_number             TEXT          NOT NULL DEFAULT '',
    provider_payment_id          TEXT          NOT NULL DEFAULT '',
    notes                        TEXT          NOT NULL DEFAULT '',
    received_at                  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    recorded_by_platform_user_id BIGINT        REFERENCES platform_users(id) ON DELETE SET NULL,
    metadata_json                JSONB         NOT NULL DEFAULT '{}',
    created_at                   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscription_payments_organization ON subscription_payments(organization_id);
CREATE INDEX idx_subscription_payments_invoice ON subscription_payments(invoice_id);
