-- Platform governance: organization-level plan & entitlement foundation.
-- Additive only. Resolve-only/shadow: no operational path enforces these yet.

-- Capability/limit catalog. Reference data, seeded below (no FK dependencies).
CREATE TABLE entitlements (
    key         TEXT PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('capability', 'limit')),
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Plan -> entitlement defaults. Additive alongside subscription_plans.features_json;
-- the entitlement resolver bridges from features_json when a plan has no rows here.
CREATE TABLE plan_entitlements (
    plan_id         BIGINT  NOT NULL REFERENCES subscription_plans(id) ON DELETE CASCADE,
    entitlement_key TEXT    NOT NULL REFERENCES entitlements(key) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,  -- capability kind
    limit_value     BIGINT,                         -- limit kind; NULL = unlimited
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plan_id, entitlement_key)
);

-- Org-level plan assignment. The resolver falls back to the org's restaurant
-- subscription (1:1 today) when no assignment exists.
CREATE TABLE organization_plan_assignments (
    organization_id              BIGINT PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id                      BIGINT NOT NULL REFERENCES subscription_plans(id),
    status                       TEXT NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('trial', 'active', 'suspended', 'cancelled')),
    assigned_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Org-level overrides: grant a capability beyond plan, or raise/lower a limit.
-- NULL columns inherit from the resolved plan entitlement.
CREATE TABLE organization_entitlement_overrides (
    organization_id             BIGINT  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    entitlement_key             TEXT    NOT NULL REFERENCES entitlements(key) ON DELETE CASCADE,
    enabled                     BOOLEAN,
    limit_value                 BIGINT,
    reason                      TEXT NOT NULL DEFAULT '',
    created_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, entitlement_key)
);

-- Deterministic catalog seed.
INSERT INTO entitlements (key, kind, description) VALUES
    ('analytics.basic',    'capability', 'Basic branch/org analytics surfaces'),
    ('analytics.advanced', 'capability', 'Advanced analytics surfaces'),
    ('custom.theme',       'capability', 'Custom theme tokens beyond presets'),
    ('multi_branch',       'capability', 'Operate more than one branch'),
    ('advanced.audit',     'capability', 'Extended audit retention and reads'),
    ('support.priority',   'capability', 'Priority operator support'),
    ('api.access',         'capability', 'Programmatic API access'),
    ('limit.branches',     'limit',      'Maximum branches per org (NULL = unlimited)'),
    ('limit.staff',        'limit',      'Maximum staff per org (NULL = unlimited)'),
    ('limit.tables',       'limit',      'Maximum tables per branch (NULL = unlimited)');
