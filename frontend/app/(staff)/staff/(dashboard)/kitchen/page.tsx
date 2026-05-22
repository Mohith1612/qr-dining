"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ordersApi } from "@/lib/api/orders"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { relativeTime } from "@/lib/format"
import { UtensilsCrossed, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"
import type { Order, OrderStatus } from "@/types/api"

const NEXT_STATUS: Partial<Record<OrderStatus, OrderStatus>> = {
  pending: "confirmed",
  confirmed: "preparing",
  preparing: "ready",
  ready: "served",
}

const NEXT_LABEL: Partial<Record<OrderStatus, string>> = {
  pending: "Confirm",
  confirmed: "Start preparing",
  preparing: "Mark ready",
  ready: "Mark served",
}

function elapsedColor(iso: string): string {
  const mins = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000)
  if (mins >= 30) return "var(--color-error)"
  if (mins >= 15) return "#d97706"
  return "var(--color-text-muted)"
}

function elapsedLabel(iso: string): string {
  const mins = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000)
  if (mins < 1) return "< 1m"
  return `${mins}m`
}

function OrderCard({
  order,
  onAdvance,
  advancing,
}: {
  order: Order
  onAdvance: (id: string, next: OrderStatus) => void
  advancing: boolean
}) {
  const next = NEXT_STATUS[order.status]
  const nextLabel = NEXT_LABEL[order.status]
  const shortId = order.id.slice(-8)

  return (
    <div
      className="rounded-2xl p-4 space-y-3"
      style={{
        backgroundColor: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        boxShadow: "var(--shadow-card)",
        borderRadius: "var(--radius-lg)",
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-0.5">
          <p className="font-mono text-xs font-semibold" style={{ color: "var(--color-text)" }}>
            #{shortId}
          </p>
          <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
            {relativeTime(order.created_at)}
          </p>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <div className="flex items-center justify-between">
        <span
          className="text-xs font-semibold tabular-nums"
          style={{ color: elapsedColor(order.created_at) }}
        >
          {elapsedLabel(order.created_at)} elapsed
        </span>

        {next && nextLabel && (
          <Button
            size="sm"
            disabled={advancing}
            onClick={() => onAdvance(order.id, next)}
            className="h-9 px-4 rounded-xl text-xs"
            style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
          >
            {advancing ? <Loader2 className="size-3.5 animate-spin" /> : nextLabel}
          </Button>
        )}
      </div>
    </div>
  )
}

const ACTIVE_STATUSES: OrderStatus[] = ["pending", "confirmed", "preparing", "ready"]

export default function KitchenPage() {
  const { branchId, token } = useStaffStore()
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [advancing, setAdvancing] = useState<string | null>(null)

  const fetchOrders = useCallback(async () => {
    if (!branchId || !token) return
    try {
      const data = await staffApi.getActiveOrders(branchId, token)
      setOrders(data)
    } catch {
      // silent refresh failure
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchOrders()
    const interval = setInterval(fetchOrders, 10_000)
    return () => clearInterval(interval)
  }, [fetchOrders])

  async function handleAdvance(orderId: string, next: OrderStatus) {
    if (!token) return
    setAdvancing(orderId)
    try {
      const updated = await ordersApi.updateStatus(orderId, next, token)
      setOrders((prev) =>
        next === "served"
          ? prev.filter((o) => o.id !== orderId)
          : prev.map((o) => (o.id === orderId ? updated : o))
      )
    } catch {
      toast.error("Couldn't update status. Please try again.")
    } finally {
      setAdvancing(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[40vh]">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

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
          <UtensilsCrossed className="size-6" style={{ color: "var(--color-text-muted)" }} aria-hidden />
        </div>
        <div className="space-y-1">
          <p className="font-medium">No active orders</p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            New orders will appear here automatically
          </p>
        </div>
      </div>
    )
  }

  return (
    <div
      className="px-4 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Order queue</h1>
        <span className="text-xs" style={{ color: "var(--color-text-muted)" }}>
          {orders.length} active
        </span>
      </div>

      {ACTIVE_STATUSES.map((status) => {
        const group = orders.filter((o) => o.status === status)
        if (group.length === 0) return null
        return (
          <section key={status} className="space-y-3">
            <h2
              className="text-xs font-semibold uppercase tracking-wider"
              style={{ color: "var(--color-text-muted)" }}
            >
              <StatusBadge status={status} className="text-xs" />
            </h2>
            {group.map((order) => (
              <OrderCard
                key={order.id}
                order={order}
                onAdvance={handleAdvance}
                advancing={advancing === order.id}
              />
            ))}
          </section>
        )
      })}
    </div>
  )
}
