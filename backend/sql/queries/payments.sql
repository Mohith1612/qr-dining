-- name: CreatePayment :one
INSERT INTO payments (
  session_id,
  order_id,
  amount,
  method,
  status,
  bill_snapshot_id,
  branch_id,
  currency,
  provider,
  provider_payment_ref,
  provider_order_ref
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $2,
    completed_at = CASE WHEN $2::payment_status = 'completed' THEN NOW() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: UpdatePaymentStatusExpected :one
UPDATE payments
SET status = $3,
    completed_at = CASE WHEN $3::payment_status = 'completed' THEN NOW() ELSE completed_at END
WHERE id = $1 AND status = $2
RETURNING *;

-- name: SettlePaymentByStaff :one
UPDATE payments
SET status = 'completed',
    settled_by_staff_id = $2,
    settled_at = NOW(),
    completed_at = NOW()
WHERE id = $1
  AND branch_id = $3
  AND status = 'requires_staff_confirmation'
RETURNING *;

-- name: ListPaymentsForSession :many
SELECT * FROM payments WHERE session_id = $1 ORDER BY initiated_at DESC;

-- name: InsertWebhookEvent :one
INSERT INTO payment_webhook_events (external_event_id, provider, event_type, payload, raw_payload, headers)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (external_event_id) DO NOTHING
RETURNING *;

-- name: GetPaymentByProviderRef :one
SELECT * FROM payments
WHERE provider = $1 AND provider_payment_ref = $2;

-- name: CreateBillSnapshot :one
INSERT INTO bill_snapshots (
  session_id,
  branch_id,
  subtotal,
  discount_amount,
  tax_amount,
  service_charge,
  tip_amount,
  total,
  currency,
  source_order_ids,
  created_by_actor
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetBillSnapshotByID :one
SELECT * FROM bill_snapshots WHERE id = $1;

-- name: SumCompletedPaymentsForSession :one
SELECT COALESCE(SUM(amount), 0)::numeric
FROM payments
WHERE session_id = $1
  AND status = 'completed';

-- name: MarkWebhookProcessed :exec
UPDATE payment_webhook_events
SET processed = TRUE, processed_at = NOW(), payment_id = $2, error_message = $3
WHERE id = $1;

-- name: ListUnprocessedWebhooks :many
SELECT * FROM payment_webhook_events
WHERE processed = FALSE
ORDER BY created_at ASC
LIMIT 50;
