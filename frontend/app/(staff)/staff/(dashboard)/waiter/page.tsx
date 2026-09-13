"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { track } from "@/lib/product-analytics/events"
import { assistanceApi } from "@/lib/api/assistance"
import { ordersApi } from "@/lib/api/orders"
import { staffApi } from "@/lib/api/staff"
import { paymentsApi } from "@/lib/api/payments"
import { StatusBadge } from "@/components/shared/StatusBadge"
import { EmptyState } from "@/components/shared/EmptyState"
import { RecoveryActionSheet } from "@/components/staff/RecoveryActionSheet"
import { canCancelPayment, isStaleRecoveryError, recoveryErrorMessage } from "@/lib/staff-recovery"
import { relativeTime, formatCurrency } from "@/lib/format"
import { CheckCircle, Loader2, UtensilsCrossed, BadgeIndianRupee, ConciergeBell } from "lucide-react"
import { toast } from "sonner"
import type { AssistanceRequest, AssistanceType, KitchenOrder, PendingPayment, PaymentMethod } from "@/types/api"

const TYPE_LABEL: Record<AssistanceType, string> = {
  waiter: "Waiter needed",
  bill:   "Bill request",
  other:  "Other",
}

const METHOD_LABEL: Record<PaymentMethod, string> = {
  cash:        "Cash",
  card:        "Card",
  card_manual: "Card",
  upi:         "UPI",
  digital:     "Digital",
}

function isUrgent(r: AssistanceRequest) {
  return r.status === "pending" && Date.now() - new Date(r.created_at).getTime() > 5 * 60 * 1000
}

function orderDisplayId(o: KitchenOrder): string {
  return o.order_operational_id ?? o.order_number_display ?? o.order_number ?? o.id.slice(0, 8)
}

function TableChip({ label }: { label: string }) {
  return (
    <span style={{
      fontSize: 10, fontWeight: 700, padding: "2px 8px",
      background: "var(--bg-elev-2)", borderRadius: "var(--rad-pill)",
      color: "var(--ink-2)", letterSpacing: "0.04em", textTransform: "uppercase",
    }}>
      {label}
    </span>
  )
}

function ActionButton({
  onClick, acting, label, tone, inkColor = "var(--accent-ink)", outline = false,
}: {
  onClick: () => void
  acting: boolean
  label: string
  tone: string
  inkColor?: string
  outline?: boolean
}) {
  return (
    <button
      disabled={acting}
      onClick={onClick}
      className="press"
      style={{
        flex: 1, height: 40, minHeight: 40,
        background: outline ? "transparent" : tone,
        color: outline ? tone : inkColor,
        border: outline ? `1px solid ${tone}` : "none",
        borderRadius: "var(--rad-md)",
        fontSize: 12, fontWeight: 600,
        cursor: acting ? "not-allowed" : "pointer",
        display: "flex", alignItems: "center", justifyContent: "center",
        opacity: acting ? 0.6 : 1,
        transition: "opacity var(--dur-fast) var(--ease)",
      }}
    >
      {acting ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : label}
    </button>
  )
}

function ServeCard({
  order, onServe, acting,
}: {
  order: KitchenOrder
  onServe: (id: string) => void
  acting: boolean
}) {
  const tableLabel = order.table_identifier ?? `Table ${order.id.slice(0, 4)}`
  return (
    <div style={{
      background: "var(--bg-elev-1)", boxShadow: "var(--shadow-1)",
      border: "1px solid var(--line-1)", borderLeft: "4px solid var(--ok)",
      borderRadius: "var(--rad-lg)", padding: "14px 16px",
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
        <TableChip label={tableLabel} />
        <span className="mono" style={{ fontSize: 11, color: "var(--ink-3)", fontWeight: 600 }}>{orderDisplayId(order)}</span>
        <span style={{ fontSize: 11, color: "var(--ink-4)", marginLeft: "auto" }}>{relativeTime(order.created_at)}</span>
      </div>

      {order.items?.length > 0 && (
        <ul style={{ listStyle: "none", margin: "0 0 12px", padding: 0, display: "flex", flexDirection: "column", gap: 4 }}>
          {order.items.map((it, i) => (
            <li key={i} style={{ fontSize: 13, color: "var(--ink-1)", display: "flex", gap: 6 }}>
              <span className="mono" style={{ color: "var(--ink-3)", fontWeight: 600 }}>{it.quantity}×</span>
              <span>{it.name}</span>
            </li>
          ))}
        </ul>
      )}

      <div style={{ display: "flex", gap: 8 }}>
        <ActionButton onClick={() => onServe(order.id)} acting={acting} label="Mark served" tone="var(--ok)" inkColor="white" />
      </div>
    </div>
  )
}

function PaymentCard({
  payment, onSettle, onCancel, acting, canCancel,
}: {
  payment: PendingPayment
  onSettle: (id: number) => void
  onCancel: (payment: PendingPayment) => void
  acting: boolean
  canCancel: boolean
}) {
  const tableLabel = payment.table_identifier || `Table ${payment.session_number}`
  return (
    <div style={{
      background: "var(--bg-elev-1)", boxShadow: "var(--shadow-1)",
      border: "1px solid var(--line-1)", borderLeft: "4px solid var(--warn)",
      borderRadius: "var(--rad-lg)", padding: "14px 16px",
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
        <TableChip label={tableLabel} />
        <span style={{ fontSize: 13, color: "var(--ink-1)", fontWeight: 500 }}>{METHOD_LABEL[payment.method] ?? payment.method}</span>
        <span style={{ fontSize: 11, color: "var(--ink-4)", marginLeft: "auto" }}>{relativeTime(payment.initiated_at)}</span>
      </div>

      <p className="serif" style={{ fontSize: 24, fontWeight: 600, color: "var(--ink-1)", margin: "0 0 12px" }}>
        {formatCurrency(payment.amount)}
      </p>

      <div style={{ display: "flex", gap: 8 }}>
        <ActionButton onClick={() => onSettle(payment.id)} acting={acting} label="Confirm collected" tone="var(--warn)" inkColor="var(--accent-ink)" />
        {canCancel && (
          <ActionButton onClick={() => onCancel(payment)} acting={acting} label="Cancel request" tone="var(--ink-3)" outline />
        )}
      </div>
      {canCancel && (
        <p style={{ fontSize: 11, color: "var(--ink-4)", margin: "8px 0 0", lineHeight: 1.4 }}>
          Cancelling withdraws the bill request — the table unfreezes and can order again.
        </p>
      )}
    </div>
  )
}

function RequestCard({
  request, onAction, acting,
}: {
  request: AssistanceRequest
  onAction: (id: number, action: "ack" | "resolve") => void
  acting: boolean
}) {
  const urgent = isUrgent(request)
  const accentColor = urgent ? "var(--alert)" : request.status === "pending" ? "var(--accent)" : "var(--ok)"
  const tableLabel = request.table_identifier ?? `Table ${request.table_id}`

  return (
    <div style={{
      background: "var(--bg-elev-1)", boxShadow: "var(--shadow-1)",
      border: "1px solid var(--line-1)", borderLeft: `4px solid ${accentColor}`,
      borderRadius: "var(--rad-lg)", padding: "14px 16px",
      animation: urgent ? "softPulse 2s ease-in-out infinite" : undefined,
    }}>
      {urgent && <p className="eyebrow" style={{ color: "var(--alert)", marginBottom: 8 }}>Urgent</p>}
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
        <TableChip label={tableLabel} />
        <span style={{ fontSize: 13, color: "var(--ink-1)", fontWeight: 500 }}>{TYPE_LABEL[request.type]}</span>
        <span style={{ fontSize: 11, color: "var(--ink-4)", marginLeft: "auto" }}>{relativeTime(request.created_at)}</span>
      </div>
      <StatusBadge status={request.status} />
      <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
        {request.status === "pending" && (
          <ActionButton onClick={() => onAction(request.id, "ack")} acting={acting} label="On my way" tone="var(--accent)" />
        )}
        {request.status === "acknowledged" && (
          <ActionButton onClick={() => onAction(request.id, "resolve")} acting={acting} label="Done ✓" tone="var(--ok)" inkColor="white" />
        )}
      </div>
    </div>
  )
}

function QueueColumn({
  icon: Icon, title, tone, count, emptyTitle, emptyDescription, children,
}: {
  icon: typeof CheckCircle
  title: string
  tone: string
  count: number
  emptyTitle: string
  emptyDescription: string
  children: React.ReactNode
}) {
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 12 }}>
        <Icon style={{ width: 15, height: 15, color: tone }} />
        <span className="eyebrow" style={{ color: tone, flexGrow: 1 }}>{title}</span>
        <span style={{
          fontSize: 10, fontWeight: 700, padding: "1px 8px",
          background: "var(--bg-elev-2)", color: tone, borderRadius: "var(--rad-pill)",
        }}>{count}</span>
      </div>
      {count === 0 ? (
        <EmptyState icon={Icon} title={emptyTitle} description={emptyDescription} />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>{children}</div>
      )}
    </div>
  )
}

export default function WaiterPage() {
  const { branchId, token, role } = useStaffStore()
  const [requests, setRequests] = useState<AssistanceRequest[]>([])
  const [readyOrders, setReadyOrders] = useState<KitchenOrder[]>([])
  const [payments, setPayments] = useState<PendingPayment[]>([])
  const [loading, setLoading] = useState(true)
  const [actingKey, setActingKey] = useState<string | null>(null)
  const [cancelTarget, setCancelTarget] = useState<PendingPayment | null>(null)
  const canCancel = canCancelPayment(role)

  const fetchAll = useCallback(async () => {
    if (!branchId || !token) return
    const [reqRes, ordersRes, payRes] = await Promise.allSettled([
      assistanceApi.getActive(branchId, token),
      staffApi.getActiveOrders(branchId, token),
      paymentsApi.listPendingForBranch(branchId, token),
    ])
    if (reqRes.status === "fulfilled") setRequests(reqRes.value)
    if (ordersRes.status === "fulfilled") setReadyOrders(ordersRes.value.filter((o) => o.status === "ready"))
    if (payRes.status === "fulfilled") setPayments(payRes.value)
    setLoading(false)
  }, [branchId, token])

  useEffect(() => {
    fetchAll()
    const interval = setInterval(fetchAll, 8_000)
    return () => clearInterval(interval)
  }, [fetchAll])

  async function handleServe(orderId: string) {
    if (!token) return
    setActingKey(`serve:${orderId}`)
    try {
      await ordersApi.updateStatus(orderId, "served", token)
      track("order_served", { order_id: orderId })
      setReadyOrders((prev) => prev.filter((o) => o.id !== orderId))
      toast.success("Marked served")
    } catch {
      toast.error("Couldn't mark served. Please try again.")
    } finally {
      setActingKey(null)
    }
  }

  async function handleSettle(paymentId: number) {
    if (!token) return
    setActingKey(`settle:${paymentId}`)
    try {
      const payment = payments.find((item) => item.id === paymentId)
      await paymentsApi.settle(paymentId, token)
      if (payment) {
        track("payment_settled", {
          session_id: payment.session_id,
          method: payment.method,
          amount: Number(payment.amount),
        })
      }
      setPayments((prev) => prev.filter((p) => p.id !== paymentId))
      toast.success("Payment confirmed")
    } catch {
      toast.error("Couldn't confirm payment. Please try again.")
    } finally {
      setActingKey(null)
    }
  }

  // Withdraw a bill request the guest changed their mind about. The session
  // unfreezes back to active server-side; the 8s poll above is what carries that
  // to every other staff device, since the WebSocket hub is guest-only.
  async function handleCancelPayment(payment: PendingPayment, reason: string) {
    if (!token) return
    try {
      await paymentsApi.cancel(payment.id, reason, token)
      track("payment_cancelled_by_staff", {
        session_id: payment.session_id,
        payment_id: payment.id,
        role: role ?? "waiter",
      })
      setPayments((prev) => prev.filter((p) => p.id !== payment.id))
      toast.success(`Bill request cancelled — ${payment.table_identifier || "the table"} can order again`)
    } catch (err) {
      toast.error(recoveryErrorMessage(err, "cancel-payment"))
      if (isStaleRecoveryError(err)) {
        // Our copy of this payment is out of date; the refetch settles it.
        fetchAll()
        return
      }
      throw err
    }
  }

  async function handleAssist(id: number, action: "ack" | "resolve") {
    if (!token) return
    setActingKey(`assist:${id}`)
    try {
      const request = requests.find((item) => item.id === id)
      const updated =
        action === "ack"
          ? await assistanceApi.acknowledge(id, token)
          : await assistanceApi.resolve(id, token)
      setRequests((prev) =>
        action === "resolve"
          ? prev.filter((r) => r.id !== id)
          : prev.map((r) => (r.id === id ? { ...r, ...updated } : r))
      )
      if (action === "ack" && request) {
        const elapsed = Math.max(0, Math.round((Date.now() - new Date(request.created_at).getTime()) / 1000))
        track("assistance_acknowledged", {
          request_id: request.id,
          assistance_type: request.type,
          ...(Number.isFinite(elapsed) ? { seconds_to_ack: elapsed } : {}),
        })
      }
      toast.success(action === "ack" ? "On your way!" : "Resolved")
    } catch {
      toast.error("Action failed. Please try again.")
    } finally {
      setActingKey(null)
    }
  }

  const activeRequests = requests.filter((r) => r.status === "pending" || r.status === "acknowledged")

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[40vh]" style={{ background: "var(--bg-base)" }}>
        <Loader2 className="animate-spin" style={{ width: 24, height: 24, color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div className="screen-enter" style={{ background: "var(--bg-base)", color: "var(--ink-1)", minHeight: "100vh" }}>
      <div style={{ padding: "24px 20px", maxWidth: 1100, margin: "0 auto" }}>
        {/* Header */}
        <p className="eyebrow">At your service</p>
        <div style={{ display: "flex", alignItems: "center", gap: 10, margin: "4px 0 0" }}>
          <h1 className="display-xl" style={{ margin: 0 }}>Service floor</h1>
          <span className="live-dot" />
        </div>

        {/* Metrics */}
        <div style={{ display: "flex", gap: 8, marginTop: 14, flexWrap: "wrap" }}>
          {[
            { label: "Ready to serve", value: readyOrders.length,    tone: "var(--ok)"     },
            { label: "Payments",       value: payments.length,        tone: "var(--warn)"   },
            { label: "Requests",       value: activeRequests.length,  tone: "var(--accent)" },
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

        {/* Three operational queues */}
        <div className="grid grid-cols-1 lg:grid-cols-3" style={{ gap: 24, marginTop: 24 }}>
          <QueueColumn
            icon={UtensilsCrossed}
            title="Ready to serve"
            tone="var(--ok)"
            count={readyOrders.length}
            emptyTitle="Nothing waiting"
            emptyDescription="Ready orders appear here to be taken out."
          >
            {readyOrders.map((o) => (
              <ServeCard key={o.id} order={o} onServe={handleServe} acting={actingKey === `serve:${o.id}`} />
            ))}
          </QueueColumn>

          <QueueColumn
            icon={BadgeIndianRupee}
            title="Payments to collect"
            tone="var(--warn)"
            count={payments.length}
            emptyTitle="No payments due"
            emptyDescription="Cash/card collections to confirm appear here."
          >
            {payments.map((p) => (
              <PaymentCard
                key={p.id}
                payment={p}
                onSettle={handleSettle}
                onCancel={setCancelTarget}
                acting={actingKey === `settle:${p.id}`}
                canCancel={canCancel}
              />
            ))}
          </QueueColumn>

          <QueueColumn
            icon={ConciergeBell}
            title="Requests"
            tone="var(--accent)"
            count={activeRequests.length}
            emptyTitle="All clear"
            emptyDescription="No active requests right now."
          >
            {activeRequests.map((r) => (
              <RequestCard key={r.id} request={r} onAction={handleAssist} acting={actingKey === `assist:${r.id}`} />
            ))}
          </QueueColumn>
        </div>
      </div>

      {cancelTarget && (
        <RecoveryActionSheet
          action="cancel-payment"
          title={`Cancel bill request — ${cancelTarget.table_identifier || cancelTarget.session_number}`}
          consequence={
            <>
              The guest&apos;s request to pay {formatCurrency(cancelTarget.amount)} is withdrawn.
              Nothing is collected, the table unfreezes and they can order again — and ask for
              the bill whenever they&apos;re ready.
            </>
          }
          submitLabel="Cancel bill request"
          onClose={() => setCancelTarget(null)}
          onSubmit={(reason) => handleCancelPayment(cancelTarget, reason)}
        />
      )}
    </div>
  )
}
