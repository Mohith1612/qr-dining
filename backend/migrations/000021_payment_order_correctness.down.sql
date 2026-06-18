DROP INDEX IF EXISTS idx_redemptions_promo_order;

ALTER TABLE promos DROP COLUMN IF EXISTS redeemed_count;

ALTER TABLE payment_webhook_events
  DROP COLUMN IF EXISTS headers,
  DROP COLUMN IF EXISTS raw_payload;

DROP INDEX IF EXISTS idx_payments_provider_ref;
DROP INDEX IF EXISTS idx_payments_branch_id;

ALTER TABLE payments
  DROP COLUMN IF EXISTS settled_at,
  DROP COLUMN IF EXISTS settled_by_staff_id,
  DROP COLUMN IF EXISTS provider_order_ref,
  DROP COLUMN IF EXISTS provider_payment_ref,
  DROP COLUMN IF EXISTS provider,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS branch_id,
  DROP COLUMN IF EXISTS bill_snapshot_id;

DROP TABLE IF EXISTS bill_snapshots;

DROP INDEX IF EXISTS idx_orders_operational_id;
DROP INDEX IF EXISTS idx_orders_scoped_idempotency;
ALTER TABLE orders
  ADD CONSTRAINT orders_idempotency_key_key UNIQUE (idempotency_key);

ALTER TABLE orders
  DROP COLUMN IF EXISTS order_operational_id,
  DROP COLUMN IF EXISTS order_number_display,
  DROP COLUMN IF EXISTS order_business_date;

DROP TABLE IF EXISTS idempotency_keys;
