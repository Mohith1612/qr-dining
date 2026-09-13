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
  provider_order_ref,
  payment_business_date,
  payment_sequence,
  payment_reference
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: NextPaymentNumber :one
INSERT INTO payment_sequences (branch_id, date, last_seq)
VALUES ($1, $2, 1)
ON CONFLICT (branch_id, date)
DO UPDATE SET last_seq = payment_sequences.last_seq + 1
RETURNING last_seq;

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

-- name: ListPaymentsForBranchByStatus :many
SELECT
    p.*,
    t.identifier AS table_identifier,
    s.session_number AS session_number
FROM payments p
JOIN sessions s ON s.id = p.session_id
JOIN tables t ON t.id = s.table_id
WHERE p.branch_id = $1 AND p.status = $2
ORDER BY p.initiated_at ASC;

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

-- name: SumCompletedPaymentsForBillSnapshot :one
SELECT COALESCE(SUM(amount), 0)::numeric
FROM payments
WHERE session_id = $1
	AND bill_snapshot_id = $2
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

-- name: ListWebhookEventsByPayment :many
SELECT * FROM payment_webhook_events
WHERE payment_id = $1
ORDER BY created_at ASC;

-- name: ListBillingReconciliationDiscrepancies :many
-- Read-only A1 detector. The latest completed payment selects the authoritative
-- snapshot, while every completed payment in the session contributes to what
-- was collected. Snapshot source_order_ids preserve the bill-time order set, so
-- a later cancellation cannot rewrite an immutable settled bill.
WITH settled AS (
  SELECT
    s.id AS session_id,
    b.organization_id,
    s.branch_id,
    s.table_id,
    authoritative_payment.bill_snapshot_id,
    bs.total AS snapshot_total,
    bs.discount_amount,
    bs.tax_amount,
    bs.service_charge,
    bs.tip_amount,
    bs.source_order_ids,
    bs.currency,
    COALESCE((
      SELECT SUM(p.amount)
      FROM payments p
      WHERE p.session_id = s.id
        AND p.status = 'completed'
    ), 0)::numeric(12,2) AS collected_total
  FROM sessions s
  JOIN branches b ON b.id = s.branch_id
  JOIN LATERAL (
    SELECT p.bill_snapshot_id, p.completed_at, p.id
    FROM payments p
    WHERE p.session_id = s.id
      AND p.status = 'completed'
      AND p.bill_snapshot_id IS NOT NULL
    ORDER BY p.completed_at DESC NULLS LAST, p.id DESC
    LIMIT 1
  ) authoritative_payment ON TRUE
  JOIN bill_snapshots bs ON bs.id = authoritative_payment.bill_snapshot_id
  WHERE s.status = 'closed'
    AND s.closed_at >= sqlc.arg(window_start)::timestamptz
    AND s.closed_at < sqlc.arg(window_end)::timestamptz
    AND NOT EXISTS (
      SELECT 1
      FROM audit_log force_close_audit
      WHERE force_close_audit.session_id = s.id
        AND force_close_audit.action = 'session.force_close'
        AND force_close_audit.result = 'success'
    )
    AND NOT EXISTS (
      SELECT 1
      FROM event_log force_close_event
      WHERE force_close_event.session_id = s.id
        AND force_close_event.event_type = 'SESSION_CLOSED'
        AND force_close_event.actor_type = 'staff'
    )
), order_totals AS (
  SELECT
    settled.session_id,
    COALESCE(SUM(ROUND(
      (COALESCE(oi.unit_price, 0) + COALESCE(modifiers.total, 0))
      * COALESCE(oi.quantity, 0),
      2
    )), 0)::numeric(12,2) AS line_subtotal
  FROM settled
  LEFT JOIN LATERAL jsonb_array_elements_text(settled.source_order_ids) source_order(id) ON TRUE
  LEFT JOIN orders o
    ON o.id::text = source_order.id
    AND o.session_id = settled.session_id
  LEFT JOIN order_items oi ON oi.order_id = o.id
  LEFT JOIN LATERAL (
    SELECT COALESCE(SUM((modifier->>'price_delta')::numeric), 0) AS total
    FROM jsonb_array_elements(COALESCE(oi.selected_modifiers_json, '[]'::jsonb)) modifier
  ) modifiers ON TRUE
  GROUP BY settled.session_id
), comparisons AS (
  SELECT
    settled.*,
    -- discount_amount is the immutable bill-time aggregate: legacy order
    -- discounts and any promo applied to the authoritative payment are already
    -- folded into it. Do not subtract promo_redemptions again; a redemption on
    -- a cancelled payment is normal. Loyalty redemption is intentionally absent:
    -- migration 35 defines it as ledger-only with any discount applied off-system.
    (
      order_totals.line_subtotal
      + settled.tax_amount
      + settled.service_charge
      + settled.tip_amount
      - settled.discount_amount
    )::numeric(12,2) AS recomputed_total
  FROM settled
  JOIN order_totals USING (session_id)
), discrepancies AS (
  SELECT
    session_id,
    organization_id,
    branch_id,
    table_id,
    bill_snapshot_id,
    'collected_vs_snapshot'::text AS comparison,
    snapshot_total::text AS expected_amount,
    collected_total::text AS actual_amount,
    (collected_total - snapshot_total)::numeric(12,2)::text AS difference,
    currency
  FROM comparisons
  -- Persisted money is NUMERIC(12,2), so exact inequality is intentional. There
  -- is no epsilon that could hide a systematic one-paise/one-rupee defect.
  WHERE collected_total <> snapshot_total

  UNION ALL

  SELECT
    session_id,
    organization_id,
    branch_id,
    table_id,
    bill_snapshot_id,
    'snapshot_vs_orders'::text AS comparison,
    recomputed_total::text AS expected_amount,
    snapshot_total::text AS actual_amount,
    (snapshot_total - recomputed_total)::numeric(12,2)::text AS difference,
    currency
  FROM comparisons
  WHERE snapshot_total <> recomputed_total
)
SELECT
  discrepancies.*,
  EXISTS (
    SELECT 1
    FROM audit_log reconciliation_audit
    WHERE reconciliation_audit.session_id = discrepancies.session_id
      AND reconciliation_audit.action = 'billing.reconciliation.discrepancy'
      AND reconciliation_audit.metadata_json->>'bill_snapshot_id' = discrepancies.bill_snapshot_id::text
      AND reconciliation_audit.metadata_json->>'comparison' = discrepancies.comparison
      AND reconciliation_audit.metadata_json->>'difference' = discrepancies.difference
  ) AS already_audited
FROM discrepancies
ORDER BY session_id, comparison;
