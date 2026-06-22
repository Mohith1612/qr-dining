DROP INDEX IF EXISTS idx_audit_log_event_reference;
ALTER TABLE audit_log DROP COLUMN IF EXISTS event_reference;
DROP SEQUENCE IF EXISTS audit_event_reference_seq;

DROP INDEX IF EXISTS idx_payment_sequences_date;
DROP INDEX IF EXISTS idx_payments_payment_reference;
DROP INDEX IF EXISTS idx_payments_branch_payment_reference_unique;
DROP TABLE IF EXISTS payment_sequences;
ALTER TABLE payments
  DROP COLUMN IF EXISTS payment_reference,
  DROP COLUMN IF EXISTS payment_sequence,
  DROP COLUMN IF EXISTS payment_business_date;

DROP INDEX IF EXISTS idx_session_sequences_date;
DROP INDEX IF EXISTS idx_sessions_session_number;
DROP INDEX IF EXISTS idx_sessions_branch_session_number_unique;
DROP TABLE IF EXISTS session_sequences;
ALTER TABLE sessions
  DROP COLUMN IF EXISTS session_number,
  DROP COLUMN IF EXISTS visit_number,
  DROP COLUMN IF EXISTS session_business_date;
