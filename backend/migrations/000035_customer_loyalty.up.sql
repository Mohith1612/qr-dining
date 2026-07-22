-- Customer loyalty foundation: spend -> points. Additive only.
-- Accounts are org-scoped (org <-> restaurant is 1:1 today; customers are
-- restaurant-scoped via UNIQUE(restaurant_id, phone_e164)).
-- Phase 1 redemption is LEDGER-ONLY: a staff-recorded points deduction.
-- It never touches bill/payment math.

-- Org-configurable earning rule: earn_rate_points per earn_rate_amount of
-- spend (e.g. 1 point per 100.00 INR). Inactive until an owner/manager
-- activates it AND the loyalty entitlement + flag gates are enabled.
CREATE TABLE organization_loyalty_programs (
    organization_id     BIGINT PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    is_active           BOOLEAN NOT NULL DEFAULT FALSE,
    earn_rate_points    BIGINT NOT NULL DEFAULT 1 CHECK (earn_rate_points > 0),
    earn_rate_amount    NUMERIC(12,2) NOT NULL DEFAULT 100.00 CHECK (earn_rate_amount > 0),
    updated_by_staff_id BIGINT REFERENCES staff(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE customer_loyalty_accounts (
    id                       BIGSERIAL PRIMARY KEY,
    customer_id              BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    organization_id          BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    points_balance           BIGINT NOT NULL DEFAULT 0 CHECK (points_balance >= 0),
    lifetime_points_earned   BIGINT NOT NULL DEFAULT 0,
    lifetime_points_redeemed BIGINT NOT NULL DEFAULT 0,
    visit_count              BIGINT NOT NULL DEFAULT 0,
    lifetime_spend           NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (customer_id, organization_id)
);

CREATE INDEX idx_loyalty_accounts_org ON customer_loyalty_accounts(organization_id);

-- Append-only points ledger. points is signed: earn > 0, redeem < 0,
-- adjustment either sign.
CREATE TABLE customer_loyalty_transactions (
    id                      BIGSERIAL PRIMARY KEY,
    account_id              BIGINT NOT NULL REFERENCES customer_loyalty_accounts(id) ON DELETE CASCADE,
    type                    TEXT NOT NULL CHECK (type IN ('earn', 'redeem', 'adjustment')),
    points                  BIGINT NOT NULL,
    amount                  NUMERIC(12,2),
    payment_id              BIGINT REFERENCES payments(id) ON DELETE SET NULL,
    session_id              UUID REFERENCES sessions(id) ON DELETE SET NULL,
    performed_by_actor_type TEXT NOT NULL DEFAULT 'system'
                            CHECK (performed_by_actor_type IN ('system', 'staff')),
    performed_by_staff_id   BIGINT REFERENCES staff(id) ON DELETE SET NULL,
    reason                  TEXT NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Earn idempotency: one earn transaction per payment, ever. A replayed
-- accrual (webhook redelivery, double hook call) violates this index and the
-- whole accrual tx rolls back.
CREATE UNIQUE INDEX idx_loyalty_txn_earn_payment
    ON customer_loyalty_transactions(payment_id)
    WHERE type = 'earn' AND payment_id IS NOT NULL;

CREATE INDEX idx_loyalty_txn_account_created
    ON customer_loyalty_transactions(account_id, created_at DESC);

-- Gating seeds. Capabilities are granted to NO plan by default; the flag is
-- catalog-seeded (cf. 000034 note) with default_enabled = FALSE.
INSERT INTO entitlements (key, kind, description) VALUES
    ('loyalty.enabled',           'capability', 'Customer loyalty accrual and balances'),
    ('loyalty.redeem',            'capability', 'Staff-recorded loyalty redemptions'),
    ('loyalty.manual_adjustment', 'capability', 'Manual loyalty point adjustments');

INSERT INTO platform_feature_flags (key, name, description, default_enabled) VALUES
    ('loyalty', 'Customer loyalty', 'Loyalty earn/redeem surfaces', FALSE);
