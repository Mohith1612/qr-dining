-- name: CreatePayment :one
INSERT INTO payments (session_id, order_id, amount, method)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $2,
    completed_at = CASE WHEN $2::payment_status = 'completed' THEN NOW() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: ListPaymentsForSession :many
SELECT * FROM payments WHERE session_id = $1 ORDER BY initiated_at DESC;

-- name: InsertWebhookEvent :one
INSERT INTO payment_webhook_events (external_event_id, provider, event_type, payload)
VALUES ($1, $2, $3, $4)
ON CONFLICT (external_event_id) DO NOTHING
RETURNING *;

-- name: MarkWebhookProcessed :exec
UPDATE payment_webhook_events
SET processed = TRUE, processed_at = NOW(), payment_id = $2, error_message = $3
WHERE id = $1;

-- name: ListUnprocessedWebhooks :many
SELECT * FROM payment_webhook_events
WHERE processed = FALSE
ORDER BY created_at ASC
LIMIT 50;
