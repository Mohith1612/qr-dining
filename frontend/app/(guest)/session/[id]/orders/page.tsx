"use client"

import { AnimatePresence, motion } from "framer-motion"
import { useOrders } from "@/hooks/useOrders"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { OrderSkeleton } from "@/components/shared/LoadingSkeleton"
import { formatCurrency, relativeTime } from "@/lib/format"
import { ClipboardList } from "lucide-react"
import { springGentle, prefersReduced } from "@/lib/motion"
import type { Order, OrderStatus } from "@/types/api"

const STATUS_STEPS: OrderStatus[] = ["pending", "confirmed", "preparing", "ready", "served"]

function OrderProgressBar({ status }: { status: OrderStatus }) {
  if (status === "cancelled") return null
  const stepIndex = STATUS_STEPS.indexOf(status)

  return (
    <div className="flex gap-1.5 mt-3" aria-label={`Order status: ${status}`}>
      {STATUS_STEPS.map((step, i) => (
        <div
          key={step}
          className="flex-1 h-1 rounded-full"
          style={{
            backgroundColor: i <= stepIndex ? "var(--color-accent)" : "var(--color-border)",
            transition: prefersReduced ? "none" : "background-color 0.4s ease",
          }}
          aria-hidden
        />
      ))}
    </div>
  )
}

function OrderCard({ order }: { order: Order }) {
  return (
    <motion.div
      layout={!prefersReduced}
      initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={prefersReduced ? {} : { opacity: 0, y: -6 }}
      transition={prefersReduced ? { duration: 0 } : springGentle}
      className="rounded-2xl p-5 space-y-1"
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        boxShadow: "var(--shadow-card)",
        borderRadius: "var(--radius-lg)",
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-0.5">
          <p
            className="text-lg font-medium leading-none"
            style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
          >
            Order
          </p>
          <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
            {relativeTime(order.created_at)}
          </p>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <OrderProgressBar status={order.status} />

      {order.status === "ready" && (
        <div
          className="mt-3 rounded-xl px-4 py-3 text-sm font-medium text-center"
          style={{
            backgroundColor: "color-mix(in oklch, var(--color-success) 15%, transparent)",
            color: "var(--color-success)",
            border: "1px solid color-mix(in oklch, var(--color-success) 30%, transparent)",
          }}
          role="alert"
          aria-live="assertive"
        >
          Your order is ready — enjoy!
        </div>
      )}

      {order.status === "cancelled" && (
        <p className="text-xs mt-2" style={{ color: "var(--color-error)" }}>
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
      <div style={{ backgroundColor: "var(--color-bg)" }} className="min-h-[60vh]">
        <EmptyState
          icon={ClipboardList}
          title="No orders yet"
          description="Your orders will appear here as they're placed."
        />
      </div>
    )
  }

  return (
    <div className="px-5 py-7 space-y-7" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}>
      {active.length > 0 && (
        <section className="space-y-3">
          <SectionHeader>Active orders</SectionHeader>
          <AnimatePresence initial={false}>
            {active.map((order) => (
              <OrderCard key={order.id} order={order} />
            ))}
          </AnimatePresence>
        </section>
      )}

      {completed.length > 0 && (
        <section className="space-y-3">
          <SectionHeader>Completed</SectionHeader>
          {completed.map((order) => (
            <OrderCard key={order.id} order={order} />
          ))}
        </section>
      )}
    </div>
  )
}
