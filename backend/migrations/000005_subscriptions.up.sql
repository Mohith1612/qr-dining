-- ============================================================
-- SUBSCRIPTION PLANS
-- Static plan definitions. Seeded at startup via seed script.
-- features_json schema: { max_branches: int (-1=unlimited),
--   max_tables: int (-1=unlimited), analytics: bool, multi_branch: bool }
-- ============================================================

CREATE TYPE plan_tier AS ENUM ('free', 'standard', 'premium');

CREATE TABLE subscription_plans (
    id            BIGSERIAL      PRIMARY KEY,
    name          TEXT           NOT NULL,
    tier          plan_tier      NOT NULL UNIQUE,
    price_monthly NUMERIC(10, 2) NOT NULL DEFAULT 0,
    features_json JSONB          NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- ============================================================
-- RESTAURANT SUBSCRIPTIONS
-- One subscription row per restaurant (UNIQUE on restaurant_id).
-- status: 'trial' | 'active' | 'expired' | 'cancelled'
-- ============================================================

CREATE TABLE restaurant_subscriptions (
    id                   BIGSERIAL    PRIMARY KEY,
    restaurant_id        BIGINT       NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    plan_id              BIGINT       NOT NULL REFERENCES subscription_plans(id),
    status               TEXT         NOT NULL DEFAULT 'trial',
    trial_ends_at        TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    current_period_end   TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(restaurant_id)
);

CREATE INDEX idx_restaurant_subscriptions_restaurant_id ON restaurant_subscriptions(restaurant_id);
