"use client"

import { useState } from "react"
import { ChevronDown, ChevronRight } from "lucide-react"
import { formatCurrency } from "@/lib/format"
import type { Bill as BillData, BillOrder } from "@/types/api"

// ─── Skeleton ────────────────────────────────────────────────────────────────

function BillSkeleton() {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10, padding: "4px 0" }}>
      {[180, 120, 160, 100].map((w, i) => (
        <div
          key={i}
          className="shimmer"
          style={{ height: 14, width: w, borderRadius: "var(--rad-sm)" }}
        />
      ))}
    </div>
  )
}

// ─── Line row ────────────────────────────────────────────────────────────────

function BillRow({
  label,
  value,
  sub,
  accent,
  muted,
  bold,
}: {
  label: string
  value: string
  sub?: boolean
  accent?: boolean
  muted?: boolean
  bold?: boolean
}) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "baseline",
        gap: 4,
        padding: sub ? "3px 0 3px 16px" : "4px 0",
      }}
    >
      <span
        style={{
          fontSize: sub ? 12 : 13,
          color: muted ? "var(--ink-3)" : accent ? "var(--accent)" : "var(--ink-2)",
          flexShrink: 0,
        }}
      >
        {label}
      </span>
      {/* Leader dots */}
      <span
        style={{
          flex: 1,
          borderBottom: "1px dotted var(--line-1)",
          marginBottom: 3,
          minWidth: 16,
        }}
      />
      <span
        style={{
          fontSize: sub ? 12 : 13,
          color: accent ? "var(--accent)" : muted ? "var(--ink-3)" : "var(--ink-1)",
          fontWeight: bold ? 600 : sub ? 400 : 500,
          flexShrink: 0,
        }}
      >
        {value}
      </span>
    </div>
  )
}

// ─── Divider ─────────────────────────────────────────────────────────────────

function Divider() {
  return (
    <div style={{ borderTop: "1px solid var(--line-1)", margin: "8px 0" }} />
  )
}

// ─── Order section ───────────────────────────────────────────────────────────

function OrderSection({ order, collapsible }: { order: BillOrder; collapsible: boolean }) {
  const [open, setOpen] = useState(true)

  const header = order.order_number ? `#${order.order_number}` : "Order"

  return (
    <div style={{ marginBottom: 4 }}>
      {collapsible ? (
        <button
          onClick={() => setOpen((p) => !p)}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6, width: "100%",
            padding: "4px 0", background: "transparent", border: "none",
            cursor: "pointer", textAlign: "left",
          }}
        >
          {open
            ? <ChevronDown size={13} style={{ color: "var(--ink-3)", flexShrink: 0 }} />
            : <ChevronRight size={13} style={{ color: "var(--ink-3)", flexShrink: 0 }} />
          }
          <span style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.06em" }}>
            {header}
          </span>
          {!open && (
            <span style={{ marginLeft: "auto", fontSize: 12, color: "var(--ink-2)", fontWeight: 500 }}>
              {formatCurrency(order.order_total)}
            </span>
          )}
        </button>
      ) : (
        <p style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
          {header}
        </p>
      )}

      {open && (
        <>
          {order.items.map((item, i) => (
            <div key={i}>
              <BillRow
                label={`${item.name} × ${item.quantity}`}
                value={formatCurrency(item.line_total)}
              />
              {item.modifiers.map((mod, j) => (
                <p key={j} style={{ fontSize: 11, color: "var(--ink-3)", paddingLeft: 16, margin: "1px 0" }}>
                  + {mod}
                </p>
              ))}
            </div>
          ))}
          {collapsible && (
            <div
              style={{
                display: "flex", justifyContent: "flex-end",
                fontSize: 12, color: "var(--ink-2)", fontWeight: 500,
                paddingTop: 4, borderTop: "1px solid var(--line-1)", marginTop: 4,
              }}
            >
              {formatCurrency(order.order_total)}
            </div>
          )}
        </>
      )}
    </div>
  )
}

// ─── Main component ──────────────────────────────────────────────────────────

interface BillBreakdownProps {
  bill: BillData | null
  loading: boolean
  error: string | null
}

export function BillBreakdown({ bill, loading, error }: BillBreakdownProps) {
  if (loading) return <BillSkeleton />

  if (error) {
    return (
      <p style={{ fontSize: 13, color: "var(--ink-3)", fontStyle: "italic" }}>
        {error}
      </p>
    )
  }

  if (!bill || bill.total === 0) {
    return (
      <p style={{ fontSize: 13, color: "var(--ink-3)", fontStyle: "italic" }}>
        Your bill will appear here once you&apos;ve ordered.
      </p>
    )
  }

  const multiOrder = bill.orders.length > 1
  const taxLabel = bill.tax_rate > 0
    ? `GST (${Math.round(bill.tax_rate * 100)}%)`
    : "GST"

  return (
    <div>
      {/* Per-order sections */}
      {bill.orders.map((order) => (
        <OrderSection key={order.order_id} order={order} collapsible={multiOrder} />
      ))}

      <Divider />

      {/* Summary */}
      {multiOrder && (
        <BillRow label="Subtotal" value={formatCurrency(bill.subtotal)} />
      )}
      {bill.tax_amount > 0 && (
        <BillRow label={taxLabel} value={formatCurrency(bill.tax_amount)} muted />
      )}
      {bill.service_charge > 0 && (
        <BillRow label="Service charge" value={formatCurrency(bill.service_charge)} muted />
      )}
      {bill.discount_amount > 0 && (
        <div
          style={{
            display: "flex", alignItems: "baseline", gap: 4, padding: "4px 0",
          }}
        >
          <span style={{ fontSize: 13, color: "var(--ok)", flexShrink: 0 }}>Discount</span>
          <span style={{ flex: 1, borderBottom: "1px dotted var(--line-1)", marginBottom: 3, minWidth: 16 }} />
          <span style={{ fontSize: 13, color: "var(--ok)", fontWeight: 500, flexShrink: 0 }}>
            −{formatCurrency(bill.discount_amount)}
          </span>
        </div>
      )}

      <Divider />

      {/* Total */}
      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", padding: "4px 0" }}>
        <span className="serif" style={{ fontSize: 18, fontWeight: 500, color: "var(--ink-1)" }}>
          Total
        </span>
        <span className="serif" style={{ fontSize: 22, fontWeight: 500, color: "var(--accent)", letterSpacing: "-0.02em" }}>
          {formatCurrency(bill.total)}
        </span>
      </div>
    </div>
  )
}
