-- Platform governance: product feature-flag targeting (global -> org -> branch).
-- Additive only. Completely separate from the 9 env-driven strict-rollout flags
-- (internal/config FeatureFlags), which remain bootstrap env booleans.

-- Flag catalog. Operators create flags here; no product flags are seeded.
CREATE TABLE platform_feature_flags (
    key             TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    default_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Global override (one per flag).
CREATE TABLE platform_flag_global_overrides (
    flag_key                    TEXT PRIMARY KEY REFERENCES platform_feature_flags(key) ON DELETE CASCADE,
    enabled                     BOOLEAN NOT NULL,
    updated_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Organization-level override.
CREATE TABLE platform_flag_organization_overrides (
    organization_id             BIGINT  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    flag_key                    TEXT    NOT NULL REFERENCES platform_feature_flags(key) ON DELETE CASCADE,
    enabled                     BOOLEAN NOT NULL,
    reason                      TEXT NOT NULL DEFAULT '',
    updated_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, flag_key)
);

-- Branch-level override (highest precedence).
CREATE TABLE platform_flag_branch_overrides (
    branch_id                   BIGINT  NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    flag_key                    TEXT    NOT NULL REFERENCES platform_feature_flags(key) ON DELETE CASCADE,
    enabled                     BOOLEAN NOT NULL,
    reason                      TEXT NOT NULL DEFAULT '',
    updated_by_platform_user_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (branch_id, flag_key)
);
