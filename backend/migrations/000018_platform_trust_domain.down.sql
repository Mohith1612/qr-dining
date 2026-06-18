DROP INDEX IF EXISTS idx_platform_audit_scope;
DROP INDEX IF EXISTS idx_platform_audit_actor;
DROP INDEX IF EXISTS idx_platform_audit_created_at;
DROP TABLE IF EXISTS platform_audit_log;

DROP INDEX IF EXISTS idx_platform_support_sessions_scope;
DROP TABLE IF EXISTS platform_support_sessions;

DROP INDEX IF EXISTS idx_platform_sessions_user_active;
DROP TABLE IF EXISTS platform_sessions;

DROP TABLE IF EXISTS platform_user_roles;
DROP TABLE IF EXISTS platform_users;
