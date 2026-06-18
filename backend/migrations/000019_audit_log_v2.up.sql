-- Phase 5: Audit Logging V2
--
-- Retention: expect ~10K rows/day at scale.
-- Future: PARTITION BY RANGE(created_at) monthly once table exceeds ~10M rows.
-- Archive cold partitions (>90 days) to object storage.
--
-- Tamper-evident chaining: row_hash and previous_hash are NULL in this phase.
-- Future: row_hash = SHA-256(canonical row fields); previous_hash = predecessor row_hash.

CREATE TYPE audit_actor_type AS ENUM (
    'platform_user',
    'organization_user',
    'staff',
    'guest',
    'system',
    'webhook'
);

CREATE TYPE audit_risk_level AS ENUM (
    'low',
    'medium',
    'high',
    'critical'
);

CREATE TYPE audit_result_type AS ENUM (
    'success',
    'failure',
    'denied'
);

CREATE TYPE audit_source_type AS ENUM (
    'web',
    'mobile',
    'pwa',
    'api',
    'webhook',
    'system'
);

CREATE TABLE audit_log (
    id               BIGSERIAL         PRIMARY KEY,
    organization_id  BIGINT,
    branch_id        BIGINT,
    restaurant_id    BIGINT,
    session_id       UUID,
    table_id         BIGINT,
    resource_type    TEXT              NOT NULL,
    resource_id      TEXT              NOT NULL DEFAULT '',
    action           TEXT              NOT NULL,
    result           audit_result_type NOT NULL DEFAULT 'success',
    actor_type       audit_actor_type  NOT NULL,
    actor_id         TEXT              NOT NULL DEFAULT '',
    actor_display    TEXT              NOT NULL DEFAULT '',
    actor_scope_json JSONB             NOT NULL DEFAULT '{}',
    request_id       TEXT              NOT NULL DEFAULT '',
    correlation_id   TEXT              NOT NULL DEFAULT '',
    idempotency_key  TEXT              NOT NULL DEFAULT '',
    ip               TEXT              NOT NULL DEFAULT '',
    user_agent       TEXT              NOT NULL DEFAULT '',
    source           audit_source_type NOT NULL DEFAULT 'api',
    before_json      JSONB,
    after_json       JSONB,
    metadata_json    JSONB             NOT NULL DEFAULT '{}',
    risk_level       audit_risk_level  NOT NULL DEFAULT 'low',
    -- Reserved for future tamper-evident chaining. Not populated in this phase.
    row_hash         TEXT,
    previous_hash    TEXT,
    created_at       TIMESTAMPTZ       NOT NULL DEFAULT NOW()
);

-- No foreign keys: audit rows must survive org/branch deletion.
-- ip stored as TEXT to avoid non-standard sqlc type overrides for INET.

CREATE OR REPLACE FUNCTION audit_log_immutable()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log rows are immutable: % is not permitted', TG_OP;
END;
$$;

CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

CREATE INDEX idx_audit_log_org_created    ON audit_log(organization_id, created_at DESC);
CREATE INDEX idx_audit_log_branch_created ON audit_log(branch_id, created_at DESC);
CREATE INDEX idx_audit_log_actor          ON audit_log(actor_type, actor_id, created_at DESC);
CREATE INDEX idx_audit_log_correlation    ON audit_log(correlation_id) WHERE correlation_id != '';
