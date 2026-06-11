ALTER TABLE payments
  ADD COLUMN subtotal        NUMERIC(12,2),
  ADD COLUMN tax_amount      NUMERIC(12,2),
  ADD COLUMN service_charge  NUMERIC(12,2),
  ADD COLUMN tip_amount      NUMERIC(12,2) NOT NULL DEFAULT 0;
