ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requested';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'provider_pending';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'requires_staff_confirmation';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'cancelled';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'partially_refunded';

ALTER TYPE payment_method ADD VALUE IF NOT EXISTS 'card_manual';
ALTER TYPE payment_method ADD VALUE IF NOT EXISTS 'upi';

CREATE TABLE idempotency_keys (
  id                     BIGSERIAL PRIMARY KEY,
  scope_type             TEXT NOT NULL,
  scope_id               TEXT NOT NULL,
  actor_type             TEXT NOT NULL,
  actor_id               TEXT NOT NULL,
  key                    TEXT NOT NULL,
  request_hash           TEXT NOT NULL,
  response_resource_type TEXT,
  response_resource_id   TEXT,
  status                 TEXT NOT NULL DEFAULT 'pending',
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at             TIMESTAMPTZ NOT NULL,
  UNIQUE(scope_type, scope_id, actor_type, actor_id, key)
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);

ALTER TABLE orders
  ADD COLUMN order_business_date DATE,
  ADD COLUMN order_number_display TEXT,
  ADD COLUMN order_operational_id TEXT;

UPDATE orders
SET order_business_date = created_at::date,
    order_number_display = COALESCE(order_number, ''),
    order_operational_id = id::text
WHERE order_business_date IS NULL;

ALTER TABLE orders
  ALTER COLUMN order_business_date SET NOT NULL,
  ALTER COLUMN order_number_display SET NOT NULL,
  ALTER COLUMN order_operational_id SET NOT NULL;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_idempotency_key_key;
CREATE INDEX idx_orders_scoped_idempotency ON orders(session_id, placed_by_participant_id, idempotency_key);
CREATE UNIQUE INDEX idx_orders_operational_id ON orders(order_operational_id);

CREATE TABLE bill_snapshots (
  id                 BIGSERIAL PRIMARY KEY,
  session_id         UUID NOT NULL REFERENCES sessions(id),
  branch_id          BIGINT NOT NULL REFERENCES branches(id),
  subtotal           NUMERIC(12,2) NOT NULL,
  discount_amount    NUMERIC(12,2) NOT NULL DEFAULT 0,
  tax_amount         NUMERIC(12,2) NOT NULL DEFAULT 0,
  service_charge     NUMERIC(12,2) NOT NULL DEFAULT 0,
  tip_amount         NUMERIC(12,2) NOT NULL DEFAULT 0,
  total              NUMERIC(12,2) NOT NULL,
  currency           TEXT NOT NULL DEFAULT 'INR',
  source_order_ids   JSONB NOT NULL DEFAULT '[]',
  created_by_actor   TEXT NOT NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bill_snapshots_session_id ON bill_snapshots(session_id, created_at DESC);

ALTER TABLE payments
  ADD COLUMN bill_snapshot_id BIGINT REFERENCES bill_snapshots(id),
  ADD COLUMN branch_id BIGINT REFERENCES branches(id),
  ADD COLUMN currency TEXT NOT NULL DEFAULT 'INR',
  ADD COLUMN provider TEXT,
  ADD COLUMN provider_payment_ref TEXT,
  ADD COLUMN provider_order_ref TEXT,
  ADD COLUMN settled_by_staff_id BIGINT REFERENCES staff(id),
  ADD COLUMN settled_at TIMESTAMPTZ;

UPDATE payments p
SET branch_id = s.branch_id
FROM sessions s
WHERE p.session_id = s.id AND p.branch_id IS NULL;

ALTER TABLE payments ALTER COLUMN branch_id SET NOT NULL;
CREATE INDEX idx_payments_branch_id ON payments(branch_id);
CREATE UNIQUE INDEX idx_payments_provider_ref ON payments(provider, provider_payment_ref) WHERE provider_payment_ref IS NOT NULL;

ALTER TABLE payment_webhook_events
  ADD COLUMN raw_payload TEXT,
  ADD COLUMN headers JSONB NOT NULL DEFAULT '{}';

ALTER TABLE promos
  ADD COLUMN redeemed_count INT NOT NULL DEFAULT 0;

UPDATE promos p
SET redeemed_count = counts.count
FROM (
  SELECT promo_id, COUNT(*)::int AS count
  FROM promo_redemptions
  GROUP BY promo_id
) counts
WHERE p.id = counts.promo_id;

CREATE UNIQUE INDEX idx_redemptions_promo_order ON promo_redemptions(promo_id, order_id);
