DROP INDEX IF EXISTS idx_order_sequences_date;
DROP TABLE IF EXISTS order_sequences;
ALTER TABLE orders DROP COLUMN IF EXISTS order_number;
ALTER TABLE branches DROP COLUMN IF EXISTS order_prefix;
