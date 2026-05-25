"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ordersApi } from "@/lib/api/orders"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { relativeTime } from "@/lib/format"
import { UtensilsCrossed, Loader2 } from "lucide-react"
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
  confirmed: "Start cooking",
  preparing: "Mark ready",
  ready: "Mark served",
}

function elapsedColor(iso: string): string {
  const mins = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000)
  if (mins >= 30) return "var(--alert)"
  if (mins >= 15) return "var(--warn)"
  return "var(--ink-3)"
}

function elapsedLabel(iso: string): string {
  const mins = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000)
  if (mins < 1) return "< 1m"
  return `${mins}m`
}

const COLUMNS: { status: OrderStatus; label: string; tone: string; toneSoft: string }[] = [
  { status: "pending",   label: "Pending",   tone: "var(--warn)",   toneSoft: "var(--warn-soft)"   },
  { status: "confirmed", label: "Confirmed", tone: "var(--info)",   toneSoft: "var(--info-soft)"   },
  { status: "preparing", label: "Preparing", tone: "var(--accent)", toneSoft: "var(--accent-soft)" },
  { status: "ready",     label: "Ready",     tone: "var(--ok)",     toneSoft: "var(--ok-soft)"     },
]

function KitchenCard({
  order, tone, toneSoft, onAdvance, advancing,
}: {
  order: Order
  tone: string
  toneSoft: string
  onAdvance: (id: string, next: OrderStatus) => void
  advancing: boolean
}) {
  const next = NEXT_STATUS[order.status]
  const nextLabel = NEXT_LABEL[order.status]
  const shortId = order.id.slice(0, 8)

  return (
    <HospitalityCard elev={2} style={{ padding: 14, borderRadius: "var(--rad-md)" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 6 }}>
        <span className="mono" style={{ fontSize: 11, color: "var(--ink-2)" }}>#{shortId}</span>
        <span style={{ fontSize: 10, color: "var(--ink-4)" }}>{relativeTime(order.created_at)}</span>
      </div>

      <p style={{ fontSize: 12, color: elapsedColor(order.created_at), marginBottom: 10 }}>
        {elapsedLabel(order.created_at)} elapsed
      </p>

      {next && nextLabel && (
        <button
          disabled={advancing}
          onClick={() => onAdvance(order.id, next)}
          className="press"
          style={{
            display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
            width: "100%", height: 36,
            background: tone, color: "var(--accent-ink)",
            border: "none", borderRadius: "var(--rad-md)",
            fontSize: 11, fontWeight: 700, letterSpacing: "0.03em",
            cursor: advancing ? "not-allowed" : "pointer",
            opacity: advancing ? 0.6 : 1,
          }}
        >
          {advancing
            ? <Loader2 className="animate-spin" style={{ width: 12, height: 12 }} />
            : nextLabel
          }
        </button>
      )}

      {/* Stale alert */}
      {Math.floor((Date.now() - new Date(order.created_at).getTime()) / 60_000) >= 15 && (
        <p style={{ fontSize: 10, color: "var(--alert)", marginTop: 6, textAlign: "center" }}>
          ⚠ Stale — check now
        </p>
      )}
    </HospitalityCard>
  )
}

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

  const pendingCount = orders.filter((o) => o.status === "pending").length
  const cookingCount = orders.filter((o) => o.status === "confirmed" || o.status === "preparing").length

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[40vh]" style={{ background: "var(--bg-base)" }}>
        <Loader2 className="animate-spin" style={{ width: 24, height: 24, color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ background: "var(--bg-base)", color: "var(--ink-1)", minHeight: "100vh", overflowX: "auto" }}>
      <div style={{ padding: "20px 16px", minWidth: 880 }}>
        {/* Header */}
        <p className="eyebrow">
          {new Date().toLocaleTimeString("en", { hour: "2-digit", minute: "2-digit" })}
        </p>
        <h1 className="serif" style={{ fontSize: 30, fontWeight: 500, color: "var(--ink-1)", margin: "4px 0 0" }}>
          Order pass · {orders.length} active
        </h1>

        {/* Metrics */}
        <div style={{ display: "flex", gap: 10, marginTop: 12, flexWrap: "wrap" }}>
          {[
            { label: "Pending",  value: pendingCount },
            { label: "Cooking",  value: cookingCount },
            { label: "Active",   value: orders.length },
          ].map(({ label, value }) => (
            <div key={label} style={{
              background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
              borderRadius: "var(--rad-pill)", padding: "7px 14px",
              display: "flex", gap: 8, alignItems: "center",
            }}>
              <p className="eyebrow" style={{ margin: 0 }}>{label}</p>
              <span className="serif" style={{ fontSize: 17, fontWeight: 600, color: "var(--ink-1)" }}>{value}</span>
            </div>
          ))}
        </div>

        {/* Kanban or empty */}
        {orders.length === 0 ? (
          <div style={{ marginTop: 40 }}>
            <EmptyState
              icon={UtensilsCrossed}
              title="No active orders"
              description="New orders will appear here automatically."
            />
          </div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 16, marginTop: 24 }}>
            {COLUMNS.map(({ status, label, tone, toneSoft }) => {
              const group = orders.filter((o) => o.status === status)
              return (
                <div key={status}>
                  {/* Column header */}
                  <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 10 }}>
                    <div style={{ width: 8, height: 8, borderRadius: "50%", background: tone, flexShrink: 0 }} />
                    <span className="eyebrow" style={{ color: tone, flexGrow: 1 }}>{label}</span>
                    <span style={{
                      fontSize: 10, fontWeight: 700, padding: "1px 7px",
                      background: toneSoft, color: tone,
                      borderRadius: "var(--rad-pill)",
                    }}>{group.length}</span>
                  </div>

                  {/* Cards */}
                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {group.length === 0 ? (
                      <div style={{
                        height: 88, border: "1px dashed var(--line-2)",
                        borderRadius: "var(--rad-md)",
                        display: "flex", alignItems: "center", justifyContent: "center",
                      }}>
                        <span style={{ fontSize: 11, color: "var(--ink-4)" }}>—</span>
                      </div>
                    ) : (
                      group.map((order) => (
                        <KitchenCard
                          key={order.id}
                          order={order}
                          tone={tone}
                          toneSoft={toneSoft}
                          onAdvance={handleAdvance}
                          advancing={advancing === order.id}
                        />
                      ))
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
