-- Payment webhook idempotency table.
-- Supports Razorpay, Cashfree, Stripe, and any future payment provider.
-- Processing flow:
--   1. INSERT with ON CONFLICT (external_event_id) DO NOTHING
--   2. If rows_affected = 0 → already processed; return 200 silently (replay protection)
--   3. If rows_affected = 1 → process the event
--   4. UPDATE SET processed=true, processed_at=NOW() after successful processing
-- This makes webhook receipt idempotent and safe to replay.

CREATE TABLE payment_webhook_events (
    id                BIGSERIAL   PRIMARY KEY,
    -- external_event_id: the payment provider's unique event or webhook ID.
    external_event_id TEXT        NOT NULL UNIQUE,
    -- provider: "razorpay" | "cashfree" | "stripe"
    provider          TEXT        NOT NULL,
    -- event_type: provider-specific event name, e.g. "payment.captured", "refund.created"
    event_type        TEXT        NOT NULL,
    -- payload: raw webhook body as received. Stored for audit and replay.
    payload           JSONB       NOT NULL,
    processed         BOOLEAN     NOT NULL DEFAULT FALSE,
    processed_at      TIMESTAMPTZ,
    payment_id        BIGINT      REFERENCES payments(id) ON DELETE SET NULL,
    -- error_message: populated if processing failed after receipt.
    error_message     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_external_id ON payment_webhook_events(external_event_id);
-- Partial index: fast queue scan for unprocessed webhooks.
CREATE INDEX idx_webhook_unprocessed ON payment_webhook_events(processed) WHERE NOT processed;
CREATE INDEX idx_webhook_created_at  ON payment_webhook_events(created_at DESC);
