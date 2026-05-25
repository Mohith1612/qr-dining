"use client"

import { useOrders } from "@/hooks/useOrders"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { EmptyState } from "@/components/shared/EmptyState"
import { formatCurrency, relativeTime } from "@/lib/format"
import { ClipboardList } from "lucide-react"
import type { Order, OrderStatus } from "@/types/api"

const STATUS_STEPS: OrderStatus[] = ["pending", "confirmed", "preparing", "ready", "served"]
const STAGE_LABELS: Record<OrderStatus, string> = {
  pending: "Sent", confirmed: "Confirmed", preparing: "Preparing", ready: "Ready", served: "Served", cancelled: "Cancelled"
}

function OrderCard({ order }: { order: Order }) {
  const stepIdx = STATUS_STEPS.indexOf(order.status as OrderStatus)
  const isLive = !["served", "cancelled"].includes(order.status)

  return (
    <div style={{
      background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
      borderRadius: "var(--rad-lg)", boxShadow: "var(--shadow-2)",
      padding: 16, marginBottom: 12,
    }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 8, marginBottom: 12 }}>
        <div>
          <p className="mono" style={{ fontSize: 12.5, color: "var(--ink-2)", marginBottom: 2 }}>
            Order · #{order.id?.toString().slice(0, 8)}
          </p>
          <p style={{ fontSize: 12, color: "var(--ink-3)" }}>{relativeTime(order.created_at)}</p>
        </div>
        <StatusBadge status={order.status as OrderStatus} />
      </div>

      {/* Segment progress bar */}
      {order.status !== "cancelled" && (
        <>
          <div className="seg-track" aria-label={`Order status: ${order.status}`}>
            {STATUS_STEPS.map((step, i) => (
              <div key={step} className={`seg${i < stepIdx ? " done" : i === stepIdx ? " active" : ""}`} aria-hidden />
            ))}
          </div>
          {/* Stage labels */}
          <div style={{ display: "flex", marginTop: 6 }}>
            {STATUS_STEPS.map((step, i) => (
              <div key={step} style={{ flex: 1, fontSize: 9, textAlign: "center", textTransform: "uppercase", letterSpacing: "0.06em", color: i <= stepIdx ? "var(--ink-2)" : "var(--ink-4)", fontWeight: i === stepIdx ? 700 : 400 }}>
                {STAGE_LABELS[step]}
              </div>
            ))}
          </div>
        </>
      )}

      {order.status === "ready" && (
        <div style={{ marginTop: 10, background: "var(--ok-soft)", border: "1px solid var(--ok)", borderRadius: "var(--rad-sm)", padding: "8px 12px", textAlign: "center", fontSize: 13, fontWeight: 600, color: "var(--ok)" }} role="alert" aria-live="assertive">
          Your order is ready — enjoy!
        </div>
      )}

      {isLive && order.status !== "ready" && (
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--ink-3)", fontSize: 12 }}>
          <span className="live-dot" />
          Updates arrive automatically
        </div>
      )}
    </div>
  )
}

export default function OrdersPage() {
  const { orders } = useOrders()

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
    <div className="screen-enter px-5 pt-6 pb-8" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Header */}
      <div style={{ marginBottom: 20 }}>
        <p className="eyebrow">From the kitchen</p>
        <h1 className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--ink-1)", margin: "6px 0 0", letterSpacing: "-0.01em" }}>
          Your active orders
        </h1>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 6, color: "var(--ink-3)", fontSize: 12 }}>
          <span className="live-dot" />
          Updates arrive automatically
        </div>
      </div>

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
  )
}
