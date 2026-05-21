"use client"

import { AnimatePresence, motion } from "framer-motion"
import { useOrders } from "@/hooks/useOrders"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { OrderSkeleton } from "@/components/shared/LoadingSkeleton"
import { formatCurrency, relativeTime } from "@/lib/format"
import { ClipboardList } from "lucide-react"
import type { Order, OrderStatus } from "@/types/api"

const STATUS_STEPS: OrderStatus[] = ["pending", "confirmed", "preparing", "ready", "served"]

const prefersReduced =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false

function OrderProgressBar({ status }: { status: OrderStatus }) {
  if (status === "cancelled") return null
  const stepIndex = STATUS_STEPS.indexOf(status)

  return (
    <div className="flex gap-1" aria-label={`Order status: ${status}`}>
      {STATUS_STEPS.map((step, i) => (
        <div
          key={step}
          className="flex-1 h-1.5 rounded-full transition-colors duration-300"
          style={{
            backgroundColor:
              i <= stepIndex ? "var(--color-accent)" : "var(--color-border)",
          }}
          aria-hidden
        />
      ))}
    </div>
  )
}

function OrderCard({ order }: { order: Order }) {
  const isActive = !["served", "cancelled"].includes(order.status)

  return (
    <motion.div
      layout={!prefersReduced}
      initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={prefersReduced ? {} : { opacity: 0, y: -8 }}
      transition={{ duration: 0.2 }}
      className="rounded-2xl p-4 space-y-3"
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        boxShadow: "var(--shadow-card)",
        borderRadius: "var(--radius-lg)",
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="font-semibold text-sm" style={{ color: "var(--color-text)" }}>
            Order
          </p>
          <p className="text-xs mt-0.5" style={{ color: "var(--color-text-muted)" }}>
            {relativeTime(order.created_at)}
          </p>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <OrderProgressBar status={order.status} />

      {order.status === "ready" && (
        <div
          className="rounded-xl px-3 py-2 text-xs font-medium text-center"
          style={{ backgroundColor: "var(--color-success)", color: "white" }}
          role="alert"
          aria-live="assertive"
        >
          Your order is ready — enjoy!
        </div>
      )}

      {order.status === "cancelled" && (
        <p className="text-xs" style={{ color: "var(--color-error)" }}>
          This order was cancelled.
        </p>
      )}
    </motion.div>
  )
}

export default function OrdersPage() {
  const { orders } = useOrders()

  const active = orders.filter((o) => !["served", "cancelled"].includes(o.status))
  const completed = orders.filter((o) => ["served", "cancelled"].includes(o.status))

  if (orders.length === 0) {
    return (
      <div
        className="min-h-[60vh] flex flex-col items-center justify-center px-6 text-center gap-4"
        style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
      >
        <div
          className="size-14 rounded-full flex items-center justify-center"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
        >
          <ClipboardList className="size-6" style={{ color: "var(--color-text-muted)" }} aria-hidden />
        </div>
        <div className="space-y-1">
          <p className="font-medium">No orders yet</p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Your orders will appear here as they're placed
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="px-4 py-6 space-y-6" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}>
      {active.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
            Active orders
          </h2>
          <AnimatePresence initial={false}>
            {active.map((order) => (
              <OrderCard key={order.id} order={order} />
            ))}
          </AnimatePresence>
        </section>
      )}

      {completed.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
            Completed
          </h2>
          {completed.map((order) => (
            <OrderCard key={order.id} order={order} />
          ))}
        </section>
      )}
    </div>
  )
}
