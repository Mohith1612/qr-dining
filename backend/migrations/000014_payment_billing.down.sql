ALTER TABLE payments
  DROP COLUMN IF EXISTS subtotal,
  DROP COLUMN IF EXISTS tax_amount,
  DROP COLUMN IF EXISTS service_charge,
  DROP COLUMN IF EXISTS tip_amount;
