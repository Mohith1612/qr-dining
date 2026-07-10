-- name: InsertAuditLog :exec
INSERT INTO audit_log (
    organization_id,
    branch_id,
    restaurant_id,
    session_id,
    table_id,
    resource_type,
    resource_id,
    action,
    result,
    actor_type,
    actor_id,
    actor_display,
    actor_scope_json,
    request_id,
    correlation_id,
    idempotency_key,
    ip,
    user_agent,
    source,
    before_json,
    after_json,
    metadata_json,
    risk_level
) VALUES (
    sqlc.narg('organization_id'),
    sqlc.narg('branch_id'),
    sqlc.narg('restaurant_id'),
    sqlc.narg('session_id'),
    sqlc.narg('table_id'),
    @resource_type,
    @resource_id,
    @action,
    @result,
    @actor_type,
    @actor_id,
    @actor_display,
    @actor_scope_json,
    @request_id,
    @correlation_id,
    @idempotency_key,
    @ip,
    @user_agent,
    @source,
    sqlc.narg('before_json'),
    sqlc.narg('after_json'),
    @metadata_json,
    @risk_level
);

-- name: ListAuditLogForBranch :many
SELECT * FROM audit_log
WHERE branch_id = @branch_id
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at < sqlc.narg('to_time'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('result')::TEXT IS NULL OR result::TEXT = sqlc.narg('result'))
  AND (sqlc.narg('source')::TEXT IS NULL OR source::TEXT = sqlc.narg('source'))
ORDER BY created_at DESC, id DESC
LIMIT 200;

-- name: ListAuditLogForOrganization :many
SELECT * FROM audit_log
WHERE organization_id = @organization_id
  AND (sqlc.narg('branch_id')::BIGINT IS NULL OR branch_id = sqlc.narg('branch_id'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at < sqlc.narg('to_time'))
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('result')::TEXT IS NULL OR result::TEXT = sqlc.narg('result'))
  AND (sqlc.narg('source')::TEXT IS NULL OR source::TEXT = sqlc.narg('source'))
ORDER BY created_at DESC, id DESC
LIMIT 200;

-- name: ListAuditLogPlatform :many
SELECT * FROM audit_log
WHERE (sqlc.narg('organization_id')::BIGINT IS NULL OR organization_id = sqlc.narg('organization_id'))
  AND (sqlc.narg('branch_id')::BIGINT IS NULL OR branch_id = sqlc.narg('branch_id'))
  AND (sqlc.narg('session_id')::UUID IS NULL OR session_id = sqlc.narg('session_id'))
  AND (sqlc.narg('actor_type')::TEXT IS NULL OR actor_type::TEXT = sqlc.narg('actor_type'))
  AND (sqlc.narg('result')::TEXT IS NULL OR result::TEXT = sqlc.narg('result'))
  AND (sqlc.narg('source')::TEXT IS NULL OR source::TEXT = sqlc.narg('source'))
  AND (sqlc.narg('from_time')::TIMESTAMPTZ IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::TIMESTAMPTZ IS NULL OR created_at < sqlc.narg('to_time'))
ORDER BY created_at DESC, id DESC
LIMIT 500;
