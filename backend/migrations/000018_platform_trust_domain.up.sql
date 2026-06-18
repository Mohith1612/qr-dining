CREATE TABLE platform_users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'disabled')),
    mfa_required  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE platform_user_roles (
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    role             TEXT NOT NULL CHECK (role IN ('super_admin', 'support_admin', 'billing_admin', 'read_only_auditor')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (platform_user_id, role)
);

CREATE TABLE platform_sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    token_hash       TEXT NOT NULL UNIQUE,
    device_name      TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL,
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX idx_platform_sessions_user_active
    ON platform_sessions(platform_user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE platform_support_sessions (
    id                            BIGSERIAL PRIMARY KEY,
    platform_user_id              BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    organization_id               BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    branch_id                     BIGINT REFERENCES branches(id) ON DELETE CASCADE,
    reason                        TEXT NOT NULL,
    approved_by_platform_user_id  BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    starts_at                     TIMESTAMPTZ NOT NULL,
    expires_at                    TIMESTAMPTZ NOT NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > starts_at)
);

CREATE INDEX idx_platform_support_sessions_scope
    ON platform_support_sessions(organization_id, branch_id, expires_at);

CREATE TABLE platform_audit_log (
    id                  BIGSERIAL PRIMARY KEY,
    platform_user_id    BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
    action              TEXT NOT NULL,
    target_type         TEXT NOT NULL,
    target_id           TEXT NOT NULL DEFAULT '',
    organization_id     BIGINT REFERENCES organizations(id) ON DELETE SET NULL,
    branch_id           BIGINT REFERENCES branches(id) ON DELETE SET NULL,
    support_session_id  BIGINT REFERENCES platform_support_sessions(id) ON DELETE SET NULL,
    request_id          TEXT NOT NULL DEFAULT '',
    payload             JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_audit_created_at ON platform_audit_log(created_at DESC);
CREATE INDEX idx_platform_audit_actor ON platform_audit_log(platform_user_id, created_at DESC);
CREATE INDEX idx_platform_audit_scope ON platform_audit_log(organization_id, branch_id, created_at DESC);
