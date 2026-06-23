-- Phase B: platform user MFA.
-- Stores the per-user TOTP secret (AES-GCM encrypted with MFA_ENCRYPTION_KEY)
-- and a small pool of bcrypt-hashed recovery codes for account recovery when
-- the authenticator device is lost.
--
-- One row per platform user. Absence of a row means MFA is not yet enrolled;
-- `mfa_required` on platform_users still drives whether enrollment is
-- required to complete login.

CREATE TABLE platform_user_mfa (
    platform_user_id  BIGINT      PRIMARY KEY REFERENCES platform_users(id) ON DELETE CASCADE,
    secret_encrypted  TEXT        NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'disabled')),
    recovery_codes    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    enrolled_at       TIMESTAMPTZ,
    last_used_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Track ephemeral MFA challenge tokens issued during login between password
-- success and TOTP verification. Held in Redis in practice; this table is the
-- audit-survivable record of MFA challenges produced for slow forensic review
-- of stuffed-credential attacks.
CREATE TABLE platform_mfa_challenges (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    platform_user_id  BIGINT      NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    challenge_hash    TEXT        NOT NULL UNIQUE,
    issued_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL,
    consumed_at       TIMESTAMPTZ,
    ip                INET,
    user_agent        TEXT
);

CREATE INDEX idx_platform_mfa_challenges_active
    ON platform_mfa_challenges(platform_user_id, expires_at)
    WHERE consumed_at IS NULL;
