ALTER TABLE branches ADD COLUMN order_prefix TEXT NOT NULL DEFAULT 'OR';
ALTER TABLE orders ADD COLUMN order_number TEXT;

CREATE TABLE order_sequences (
  branch_id   BIGINT NOT NULL REFERENCES branches(id),
  date        DATE   NOT NULL,
  last_seq    INT    NOT NULL DEFAULT 1000,
  PRIMARY KEY (branch_id, date)
);

CREATE INDEX idx_order_sequences_date ON order_sequences(date);
