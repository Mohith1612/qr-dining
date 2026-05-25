"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ordersApi } from "@/lib/api/orders"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
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
  if (mins >= 30) return "var(--color-elapsed-critical)"
  if (mins >= 15) return "var(--color-elapsed-warning)"
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
  const shortId = order.id.slice(0, 8)

  return (
    <HospitalityCard style={{ padding: "1rem", gap: undefined }}>
      <div className="space-y-3">
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
              className="h-8 px-4 rounded-xl text-xs"
              style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
            >
              {advancing ? <Loader2 className="size-3.5 animate-spin" /> : nextLabel}
            </Button>
          )}
        </div>
      </div>
    </HospitalityCard>
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
      <div style={{ backgroundColor: "var(--color-bg)" }} className="min-h-[60vh]">
        <EmptyState
          icon={UtensilsCrossed}
          title="No active orders"
          description="New orders will appear here automatically."
        />
      </div>
    )
  }

  return (
    <div
      className="px-5 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="flex items-center justify-between">
        <h1
          className="text-2xl font-medium"
          style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
        >
          Order queue
        </h1>
        <span
          className="text-xs font-medium px-2.5 py-1 rounded-full"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)", color: "var(--color-text-muted)" }}
        >
          {orders.length} active
        </span>
      </div>

      {/* Desktop: 2-column grid for wider screens */}
      <div className="md:grid md:grid-cols-2 md:gap-6 space-y-6 md:space-y-0">
        {ACTIVE_STATUSES.map((status) => {
          const group = orders.filter((o) => o.status === status)
          if (group.length === 0) return null
          return (
            <section key={status} className="space-y-3">
              <div className="flex items-center gap-2">
                <StatusBadge status={status} />
                <span className="text-xs" style={{ color: "var(--color-text-muted)" }}>
                  {group.length}
                </span>
              </div>
              <div className="space-y-2">
                {group.map((order) => (
                  <OrderCard
                    key={order.id}
                    order={order}
                    onAdvance={handleAdvance}
                    advancing={advancing === order.id}
                  />
                ))}
              </div>
            </section>
          )
        })}
      </div>
    </div>
  )
}
