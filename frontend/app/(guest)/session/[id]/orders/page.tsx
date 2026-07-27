"use client"

import { useOrders } from "@/hooks/useOrders"
import { useSession } from "@/hooks/useSession"
import { OrderSkeleton } from "@/components/shared/LoadingSkeleton"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { EmptyState } from "@/components/shared/EmptyState"
import { formatCurrency, relativeTime } from "@/lib/format"
import { ClipboardList, Check } from "lucide-react"
import type { Order, OrderStatus } from "@/types/api"

const STATUS_STEPS: OrderStatus[] = ["pending", "confirmed", "preparing", "ready", "served"]
const STAGE_LABELS: Record<OrderStatus, string> = {
  pending: "Sent", confirmed: "Confirmed", preparing: "Preparing", ready: "Ready", served: "Served", cancelled: "Cancelled"
}

function OrderCard({ order }: { order: Order }) {
  const stepIdx = STATUS_STEPS.indexOf(order.status as OrderStatus)
  const isLive = !["served", "cancelled"].includes(order.status)
  const progress = stepIdx <= 0 ? 0 : stepIdx / (STATUS_STEPS.length - 1)

  return (
    <div style={{
      background: "var(--bg-elev-1)", border: "1px solid var(--line-1)",
      borderRadius: "var(--rad-lg)", boxShadow: "var(--shadow-2)",
      padding: 16, marginBottom: 12,
    }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 8, marginBottom: 16 }}>
        <div>
          <p className="mono" style={{ fontSize: 12.5, color: "var(--ink-2)", marginBottom: 2 }}>
            {order.order_number ? `Order · #${order.order_number}` : `Order · #${order.id.slice(0, 6).toUpperCase()}`}
          </p>
          <p style={{ fontSize: 12, color: "var(--ink-3)" }}>{relativeTime(order.created_at)}</p>
        </div>
        <StatusBadge status={order.status as OrderStatus} />
      </div>

      {/* Node stepper */}
      {order.status !== "cancelled" && (
        <div style={{ position: "relative", padding: "0 2px", marginBottom: 14 }} aria-label={`Order status: ${order.status}`}>
          {/* track + progress line */}
          <div style={{ position: "absolute", left: 14, right: 14, top: 14, height: 2, background: "var(--line-2)", borderRadius: 2 }} aria-hidden />
          <div style={{ position: "absolute", left: 14, top: 14, height: 2, background: "var(--accent)", borderRadius: 2, width: `calc((100% - 28px) * ${progress})`, transition: "width var(--dur-slow) var(--ease-out)" }} aria-hidden />
          <div style={{ display: "flex", position: "relative" }}>
            {STATUS_STEPS.map((step, i) => {
              const done = i < stepIdx
              const active = i === stepIdx
              return (
                <div key={step} style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: 7 }}>
                  <span style={{
                    width: 28, height: 28, borderRadius: "50%",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    background: done || active ? "var(--accent)" : "var(--bg-elev-1)",
                    border: done || active ? "none" : "1px solid var(--line-2)",
                    boxShadow: active ? "0 0 0 4px var(--accent-soft)" : "none",
                    color: "var(--accent-ink)",
                  }} aria-hidden>
                    {done ? (
                      <Check size={14} />
                    ) : active ? (
                      <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--accent-ink)", animation: "softPulse 1.4s ease-in-out infinite" }} />
                    ) : null}
                  </span>
                  <span style={{ fontSize: 9.5, textTransform: "uppercase", letterSpacing: "0.04em", textAlign: "center", lineHeight: 1.2, color: i <= stepIdx ? "var(--ink-2)" : "var(--ink-4)", fontWeight: active ? 700 : 500 }}>
                    {STAGE_LABELS[step]}
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {order.status === "ready" && (
        <div style={{ marginBottom: 12, background: "var(--ok-soft)", border: "1px solid var(--ok)", borderRadius: "var(--rad-sm)", padding: "8px 12px", textAlign: "center", fontSize: 13, fontWeight: 600, color: "var(--ok)" }} role="alert" aria-live="assertive">
          Your order is ready — enjoy!
        </div>
      )}

      {/* Summary footer */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", paddingTop: 12, borderTop: "1px solid var(--line-1)" }}>
        {isLive && order.status !== "ready" ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 12.5 }}>
            <span className="live-dot" /> Updates arrive automatically
          </span>
        ) : (
          <span style={{ fontSize: 12.5, color: "var(--ink-3)" }}>{STAGE_LABELS[order.status as OrderStatus] ?? ""}</span>
        )}
        <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)", fontVariantNumeric: "tabular-nums" }}>
          {formatCurrency(order.total_amount)}
        </span>
      </div>
    </div>
  )
}

export default function OrdersPage() {
  const { session } = useSession()
  const { orders } = useOrders()

  if (!session) return <OrderSkeleton />

  const active = orders.filter((o) => !["served", "cancelled"].includes(o.status))
  const completed = orders.filter((o) => ["served", "cancelled"].includes(o.status))

  if (orders.length === 0) {
    return (
      <div style={{ minHeight: "70vh", display: "flex", flexDirection: "column" }}>
        <EmptyState
          icon={ClipboardList}
          eyebrow="From the kitchen"
          title="No orders yet"
          description="Your orders will appear here as they're placed."
        />
      </div>
    )
  }

  return (
    <div className="screen-enter" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Header */}
      <div className="page-glow" style={{ padding: "24px 20px 16px", marginBottom: 4 }}>
        <p className="eyebrow">From the kitchen</p>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          Your orders
        </h1>
        <p style={{ margin: "0 0 8px", color: "var(--ink-2)", fontSize: 13.5 }}>Placed orders update in real time as the kitchen works.</p>
        <div style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--ink-2)", fontSize: 12.5 }}>
          <span className="live-dot" />
          Live updates
        </div>
      </div>
      <div style={{ padding: "0 20px" }}>

      {active.length > 0 && (
        <section style={{ marginBottom: 28 }}>
          <p className="eyebrow" style={{ marginBottom: 12 }}>Active</p>
          {active.map((order) => <OrderCard key={order.id} order={order} />)}
        </section>
      )}

      {completed.length > 0 && (
        <section>
          <p className="eyebrow" style={{ marginBottom: 12 }}>Completed</p>
          {completed.map((order) => (
            <div key={order.id} style={{ opacity: 0.6 }}>
              <OrderCard order={order} />
            </div>
          ))}
        </section>
      )}
      </div>
    </div>
  )
}
