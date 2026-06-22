"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ordersApi } from "@/lib/api/orders"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { relativeTime } from "@/lib/format"
import { UtensilsCrossed, Loader2 } from "lucide-react"
import { KitchenSkeleton } from "@/components/shared/LoadingSkeleton"
import { toast } from "sonner"
import type { Order, OrderStatus } from "@/types/api"

const NEXT_STATUS: Partial<Record<OrderStatus, OrderStatus>> = {
  pending:   "confirmed",
  confirmed: "preparing",
  preparing: "ready",
  ready:     "served",
}

const NEXT_LABEL: Partial<Record<OrderStatus, string>> = {
  pending:   "Confirm",
  confirmed: "Start cooking",
  preparing: "Mark ready",
  ready:     "Mark served",
}

function elapsedMins(iso: string): number {
  return Math.floor((Date.now() - new Date(iso).getTime()) / 60_000)
}

function elapsedColor(mins: number): string {
  if (mins >= 30) return "var(--alert)"
  if (mins >= 15) return "var(--warn)"
  return "var(--ink-3)"
}

function elapsedLabel(mins: number): string {
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
  const displayId = order.order_operational_id ?? order.order_number_display ?? order.order_number ?? order.id.slice(0, 8)
  const mins = elapsedMins(order.created_at)
  const isStale = mins >= 15

  return (
    <HospitalityCard elev={2} style={{ padding: 14, borderRadius: "var(--rad-md)", borderLeft: `3px solid ${tone}` }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 8 }}>
        <span className="mono" style={{ fontSize: 11, color: "var(--ink-2)", fontWeight: 600 }}>{displayId}</span>
        <span style={{ fontSize: 10, color: "var(--ink-4)" }}>{relativeTime(order.created_at)}</span>
      </div>

      {/* Elapsed time */}
      <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: 10 }}>
        {isStale && (
          <span style={{
            width: 6, height: 6, borderRadius: "50%",
            background: elapsedColor(mins), flexShrink: 0,
            animation: "softPulse 1.5s ease-in-out infinite",
          }} />
        )}
        <p style={{ fontSize: 12, color: elapsedColor(mins), fontWeight: isStale ? 600 : 400 }}>
          {elapsedLabel(mins)} elapsed
        </p>
      </div>

      {/* Stale alert banner */}
      {mins >= 15 && (
        <div style={{
          background: mins >= 30 ? "var(--alert-soft)" : "var(--warn-soft)",
          border: `1px solid ${mins >= 30 ? "var(--alert)" : "var(--warn)"}`,
          borderRadius: "var(--rad-sm)",
          padding: "5px 8px",
          fontSize: 11, fontWeight: 600,
          color: mins >= 30 ? "var(--alert)" : "var(--warn)",
          textAlign: "center",
          marginBottom: 10,
        }}>
          {mins >= 30 ? "Critical — check now" : "Needs attention"}
        </div>
      )}

      {/* Advance button */}
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
            fontSize: 11, fontWeight: 700, letterSpacing: "0.04em",
            cursor: advancing ? "not-allowed" : "pointer",
            opacity: advancing ? 0.6 : 1,
            transition: "opacity var(--dur-fast) var(--ease)",
          }}
        >
          {advancing
            ? <Loader2 className="animate-spin" style={{ width: 12, height: 12 }} />
            : nextLabel
          }
        </button>
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
      // silent refresh
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

  const pendingCount  = orders.filter((o) => o.status === "pending").length
  const cookingCount  = orders.filter((o) => o.status === "confirmed" || o.status === "preparing").length

  if (loading) return <KitchenSkeleton />

  return (
    <div className="screen-enter" style={{ position: "relative", background: "var(--bg-base)", color: "var(--ink-1)", minHeight: "100vh" }}>
      <div style={{ overflowX: "auto" }}>
      <div style={{ padding: "24px 20px", minWidth: 900 }}>
        {/* Header */}
        <p className="eyebrow">
          {new Date().toLocaleTimeString("en", { hour: "2-digit", minute: "2-digit" })}
        </p>
        <div style={{ display: "flex", alignItems: "center", gap: 10, margin: "4px 0 0" }}>
          <h1 className="display-xl" style={{ margin: 0 }}>
            Order pass
          </h1>
          <span style={{ fontSize: 13, color: "var(--ink-3)", fontWeight: 400 }}>· {orders.length} active</span>
        </div>

        {/* Metrics */}
        <div style={{ display: "flex", gap: 8, marginTop: 14, flexWrap: "wrap" }}>
          {[
            { label: "Pending", value: pendingCount,  tone: "var(--warn)"   },
            { label: "Cooking", value: cookingCount,  tone: "var(--accent)" },
            { label: "Active",  value: orders.length, tone: "var(--ink-2)"  },
          ].map(({ label, value, tone }) => (
            <div key={label} style={{
              background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
              borderRadius: "var(--rad-pill)", padding: "6px 14px",
              display: "flex", gap: 8, alignItems: "center",
            }}>
              <p className="eyebrow" style={{ margin: 0 }}>{label}</p>
              <span className="serif" style={{ fontSize: 17, fontWeight: 600, color: tone }}>{value}</span>
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
                    <div style={{ width: 10, height: 10, borderRadius: "50%", background: tone, flexShrink: 0 }} />
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
                        height: 100, border: "1px dashed var(--line-2)",
                        borderRadius: "var(--rad-md)",
                        display: "flex", alignItems: "center", justifyContent: "center",
                      }}>
                        <span className="serif" style={{ fontSize: 20, color: "var(--ink-4)", opacity: 0.4 }}>—</span>
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
      {/* Right-edge scroll fade indicator */}
      <div aria-hidden style={{ position: "absolute", top: 0, right: 0, bottom: 0, width: 48, background: "linear-gradient(to right, transparent, var(--bg-base))", pointerEvents: "none", zIndex: 10 }} />
    </div>
  )
}
