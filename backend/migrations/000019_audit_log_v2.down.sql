DROP TRIGGER IF EXISTS trg_audit_log_immutable ON audit_log;
DROP FUNCTION IF EXISTS audit_log_immutable();
DROP TABLE IF EXISTS audit_log;
DROP TYPE IF EXISTS audit_source_type;
DROP TYPE IF EXISTS audit_result_type;
DROP TYPE IF EXISTS audit_risk_level;
DROP TYPE IF EXISTS audit_actor_type;
