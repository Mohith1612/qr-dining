-- name: GetPlatformMFA :one
SELECT * FROM platform_user_mfa WHERE platform_user_id = $1;

-- name: UpsertPlatformMFAPending :one
INSERT INTO platform_user_mfa (platform_user_id, secret_encrypted, status, recovery_codes)
VALUES ($1, $2, 'pending', '[]'::jsonb)
ON CONFLICT (platform_user_id) DO UPDATE
SET secret_encrypted = EXCLUDED.secret_encrypted,
    status = 'pending',
    recovery_codes = '[]'::jsonb,
    updated_at = NOW()
RETURNING *;

-- name: ActivatePlatformMFA :one
UPDATE platform_user_mfa
SET status = 'active',
    recovery_codes = $2,
    enrolled_at = NOW(),
    last_used_at = NOW(),
    updated_at = NOW()
WHERE platform_user_id = $1 AND status = 'pending'
RETURNING *;

-- name: DisablePlatformMFA :exec
UPDATE platform_user_mfa
SET status = 'disabled',
    recovery_codes = '[]'::jsonb,
    updated_at = NOW()
WHERE platform_user_id = $1;

-- name: TouchPlatformMFAUse :exec
UPDATE platform_user_mfa
SET last_used_at = NOW(),
    updated_at = NOW()
WHERE platform_user_id = $1;

-- name: ConsumeRecoveryCodes :exec
UPDATE platform_user_mfa
SET recovery_codes = $2,
    last_used_at = NOW(),
    updated_at = NOW()
WHERE platform_user_id = $1;

-- name: CreatePlatformMFAChallenge :one
INSERT INTO platform_mfa_challenges (platform_user_id, challenge_hash, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPlatformMFAChallengeByHash :one
SELECT * FROM platform_mfa_challenges
WHERE challenge_hash = $1
  AND consumed_at IS NULL
  AND expires_at > NOW();

-- name: ConsumePlatformMFAChallenge :exec
UPDATE platform_mfa_challenges
SET consumed_at = NOW()
WHERE challenge_hash = $1 AND consumed_at IS NULL;
