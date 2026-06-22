-- Phase 8: Operational UX Cleanup
--
-- Add human-readable operational references while keeping UUIDs / BIGSERIALs as
-- primary keys. References are intended for staff and support workflows only.

ALTER TABLE sessions
  ADD COLUMN session_business_date DATE,
  ADD COLUMN visit_number INT,
  ADD COLUMN session_number TEXT;

WITH session_backfill AS (
  SELECT
    s.id,
    (s.created_at AT TIME ZONE b.timezone)::date AS business_date,
    ROW_NUMBER() OVER (
      PARTITION BY s.branch_id, (s.created_at AT TIME ZONE b.timezone)::date
      ORDER BY s.created_at, s.id::text
    )::int AS seq,
    b.branch_code
  FROM sessions s
  JOIN branches b ON b.id = s.branch_id
)
UPDATE sessions s
SET session_business_date = session_backfill.business_date,
    visit_number = session_backfill.seq,
    session_number = session_backfill.branch_code || '-S-' || to_char(session_backfill.business_date, 'YYYYMMDD') || '-' || lpad(session_backfill.seq::text, 3, '0')
FROM session_backfill
WHERE session_backfill.id = s.id;

ALTER TABLE sessions
  ALTER COLUMN session_business_date SET NOT NULL,
  ALTER COLUMN visit_number SET NOT NULL,
  ALTER COLUMN session_number SET NOT NULL;

CREATE TABLE session_sequences (
  branch_id BIGINT NOT NULL REFERENCES branches(id),
  date      DATE   NOT NULL,
  last_seq  INT    NOT NULL DEFAULT 0,
  PRIMARY KEY (branch_id, date)
);

INSERT INTO session_sequences (branch_id, date, last_seq)
SELECT branch_id, session_business_date, MAX(visit_number)
FROM sessions
GROUP BY branch_id, session_business_date;

CREATE UNIQUE INDEX idx_sessions_branch_session_number_unique
  ON sessions(branch_id, session_number);
CREATE INDEX idx_sessions_session_number
  ON sessions(session_number);
CREATE INDEX idx_session_sequences_date
  ON session_sequences(date);

ALTER TABLE payments
  ADD COLUMN payment_business_date DATE,
  ADD COLUMN payment_sequence INT,
  ADD COLUMN payment_reference TEXT;

WITH payment_backfill AS (
  SELECT
    p.id,
    (p.initiated_at AT TIME ZONE b.timezone)::date AS business_date,
    ROW_NUMBER() OVER (
      PARTITION BY p.branch_id, (p.initiated_at AT TIME ZONE b.timezone)::date
      ORDER BY p.initiated_at, p.id
    )::int AS seq,
    b.branch_code
  FROM payments p
  JOIN branches b ON b.id = p.branch_id
)
UPDATE payments p
SET payment_business_date = payment_backfill.business_date,
    payment_sequence = payment_backfill.seq,
    payment_reference = payment_backfill.branch_code || '-PAY-' || to_char(payment_backfill.business_date, 'YYYYMMDD') || '-' || lpad(payment_backfill.seq::text, 4, '0')
FROM payment_backfill
WHERE payment_backfill.id = p.id;

ALTER TABLE payments
  ALTER COLUMN payment_business_date SET NOT NULL,
  ALTER COLUMN payment_sequence SET NOT NULL,
  ALTER COLUMN payment_reference SET NOT NULL;

CREATE TABLE payment_sequences (
  branch_id BIGINT NOT NULL REFERENCES branches(id),
  date      DATE   NOT NULL,
  last_seq  INT    NOT NULL DEFAULT 0,
  PRIMARY KEY (branch_id, date)
);

INSERT INTO payment_sequences (branch_id, date, last_seq)
SELECT branch_id, payment_business_date, MAX(payment_sequence)
FROM payments
GROUP BY branch_id, payment_business_date;

CREATE UNIQUE INDEX idx_payments_branch_payment_reference_unique
  ON payments(branch_id, payment_reference);
CREATE INDEX idx_payments_payment_reference
  ON payments(payment_reference);
CREATE INDEX idx_payment_sequences_date
  ON payment_sequences(date);

CREATE SEQUENCE audit_event_reference_seq;

ALTER TABLE audit_log
  ADD COLUMN event_reference TEXT NOT NULL DEFAULT (
    'AUD-' || to_char(NOW(), 'YYYYMMDD') || '-' || lpad(nextval('audit_event_reference_seq')::text, 8, '0')
  );

UPDATE audit_log
SET event_reference = 'AUD-' || to_char(created_at, 'YYYYMMDD') || '-' || lpad(id::text, 8, '0');

CREATE UNIQUE INDEX idx_audit_log_event_reference
  ON audit_log(event_reference);
