"use client"

import { useEffect, useState, useCallback, useRef } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { paymentsApi } from "@/lib/api/payments"
import { sessionsApi } from "@/lib/api/sessions"
import { RecoveryActionSheet } from "@/components/staff/RecoveryActionSheet"
import {
  canCancelPayment,
  canForceCloseSession,
  isNonTerminalPayment,
  isStaleRecoveryError,
  recoveryErrorMessage,
} from "@/lib/staff-recovery"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { EmptyState } from "@/components/shared/EmptyState"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users } from "lucide-react"
import { toast } from "sonner"
import { track } from "@/lib/product-analytics/events"
import type { SessionWithTable as Session, PendingPayment } from "@/types/api"

export function SessionsTab() {
  const { branchId, token, role } = useStaffStore()
  const [sessions, setSessions] = useState<Session[]>([])
  // Non-terminal payments for this branch, keyed by session. A session with one
  // is frozen in payment_pending; cancelling it is what unfreezes the table.
  const [payments, setPayments] = useState<Record<string, PendingPayment>>({})
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [cancelTarget, setCancelTarget] = useState<{ session: Session; payment: PendingPayment } | null>(null)
  const [closeTarget, setCloseTarget] = useState<Session | null>(null)
  const loadedOnce = useRef(false)

  const canCancel = canCancelPayment(role)
  const canForceClose = canForceCloseSession(role)

  const fetchBoard = useCallback(async () => {
    if (!branchId || !token) return
    setRefreshing(true)
    // Both statuses the branch payments route will serve. Everything else is
    // terminal and cannot hold a session frozen.
    const [sessRes, staffConfirmRes, providerRes] = await Promise.allSettled([
      staffApi.getActiveSessions(branchId, token),
      paymentsApi.listPendingForBranch(branchId, token, "requires_staff_confirmation"),
      paymentsApi.listPendingForBranch(branchId, token, "provider_pending"),
    ])

    if (sessRes.status === "fulfilled") {
      setSessions(sessRes.value)
    } else if (!loadedOnce.current) {
      toast.error("Couldn't load sessions.")
    }

    const bySession: Record<string, PendingPayment> = {}
    for (const res of [staffConfirmRes, providerRes]) {
      if (res.status !== "fulfilled") continue
      for (const p of res.value) {
        if (isNonTerminalPayment(p.status)) bySession[p.session_id] = p
      }
    }
    if (staffConfirmRes.status === "fulfilled" || providerRes.status === "fulfilled") {
      setPayments(bySession)
    }

    loadedOnce.current = true
    setLoading(false)
    setRefreshing(false)
  }, [branchId, token])

  // Poll on the same 8s cadence as the waiter board. There is no staff
  // WebSocket — the hub only issues tickets to guest participants — so polling
  // is how a second staff device learns that this one cancelled or closed.
  useEffect(() => {
    fetchBoard()
    const interval = setInterval(fetchBoard, 8_000)
    return () => clearInterval(interval)
  }, [fetchBoard])

  async function handleCancelPayment(session: Session, payment: PendingPayment, reason: string) {
    if (!token) return
    try {
      await paymentsApi.cancel(payment.id, reason, token)
      track("payment_cancelled_by_staff", {
        session_id: session.id,
        payment_id: payment.id,
        role: role ?? "manager",
      })
      setPayments((prev) => {
        const next = { ...prev }
        delete next[session.id]
        return next
      })
      setSessions((prev) => prev.map((s) => (s.id === session.id ? { ...s, status: "active" } : s)))
      toast.success(`Bill request cancelled — ${tableLabel(session)} can order again`)
      fetchBoard()
    } catch (err) {
      toast.error(recoveryErrorMessage(err, "cancel-payment"))
      if (isStaleRecoveryError(err)) {
        fetchBoard()
        return
      }
      throw err
    }
  }

  async function handleForceClose(session: Session, reason: string) {
    if (!token) return
    try {
      const result = await sessionsApi.forceClose(session.id, reason, token)
      track("session_force_closed", {
        session_id: session.id,
        cancelled_payment_count: result.cancelled_payment_ids?.length ?? 0,
        role: role ?? "manager",
      })
      setSessions((prev) => prev.filter((s) => s.id !== session.id))
      setPayments((prev) => {
        const next = { ...prev }
        delete next[session.id]
        return next
      })
      const stranded = result.stranded_payment_ids?.length ?? 0
      toast.success(`${tableLabel(session)} closed and freed`)
      if (stranded > 0) {
        // The state machine refused to move these; they need a human.
        toast.warning(`${stranded} payment${stranded === 1 ? "" : "s"} on that table couldn't be cancelled. Check with a manager.`)
      }
      fetchBoard()
    } catch (err) {
      toast.error(recoveryErrorMessage(err, "force-close"))
      if (isStaleRecoveryError(err)) {
        fetchBoard()
        return
      }
      throw err
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
          {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={fetchBoard}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6,
            fontSize: 12, color: "var(--ink-3)",
            padding: "6px 10px", borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
          }}
          aria-label="Refresh sessions"
        >
          <RefreshCw size={12} className={refreshing ? "animate-spin" : undefined} aria-hidden />
          Refresh
        </button>
      </div>

      {sessions.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No active sessions"
          description="Active sessions will appear here."
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {sessions.map((s) => {
            const payment = payments[s.id]
            const showCancel = canCancel && payment != null
            return (
              <HospitalityCard key={s.id} elev={1} style={{ padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                  <div style={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 3 }}>
                    <p className="mono" style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-1)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {s.session_number ?? `Session ${s.id.slice(0, 8)}`}
                    </p>
                    <p style={{ fontSize: 12, color: "var(--ink-3)" }}>
                      {tableLabel(s)} · Visit {s.visit_number ?? "—"} · {relativeTime(s.created_at)}
                    </p>
                  </div>
                  <span
                    style={{
                      fontSize: 11, fontWeight: 500,
                      padding: "3px 10px", borderRadius: "var(--rad-pill)",
                      background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                      color: "var(--ink-3)", flexShrink: 0,
                      letterSpacing: "0.03em", textTransform: "uppercase",
                    }}
                  >
                    {s.status}
                  </span>
                </div>

                {payment && (
                  <p style={{ fontSize: 12, color: "var(--warn)", margin: "8px 0 0" }}>
                    {formatCurrency(payment.amount)} awaiting settlement — the cart is frozen until it is settled or cancelled.
                  </p>
                )}

                {(showCancel || canForceClose) && (
                  <div style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
                    {showCancel && (
                      <button
                        onClick={() => setCancelTarget({ session: s, payment })}
                        className="press"
                        style={{ ...rowActionStyle, color: "var(--ink-2)", borderColor: "var(--line-3)" }}
                      >
                        Cancel bill request
                      </button>
                    )}
                    {canForceClose && (
                      <button
                        onClick={() => setCloseTarget(s)}
                        className="press"
                        style={{ ...rowActionStyle, color: "var(--alert)", borderColor: "var(--alert)" }}
                      >
                        Force-close table
                      </button>
                    )}
                  </div>
                )}
              </HospitalityCard>
            )
          })}
        </div>
      )}

      {cancelTarget && (
        <RecoveryActionSheet
          action="cancel-payment"
          title={`Cancel bill request — ${tableLabel(cancelTarget.session)}`}
          consequence={
            <>
              The guest&apos;s request to pay {formatCurrency(cancelTarget.payment.amount)} is withdrawn.
              Nothing is collected, the table unfreezes and they can order again — and ask for
              the bill whenever they&apos;re ready.
            </>
          }
          submitLabel="Cancel bill request"
          onClose={() => setCancelTarget(null)}
          onSubmit={(reason) => handleCancelPayment(cancelTarget.session, cancelTarget.payment, reason)}
        />
      )}

      {closeTarget && (
        <RecoveryActionSheet
          action="force-close"
          title={`Force-close ${tableLabel(closeTarget)}`}
          consequence={
            <>
              This ends the table now, without the guests closing it. The table is freed for the
              next party, every guest at it is signed out immediately, and any bill still
              outstanding is written off as cancelled — no money is recorded as collected.
            </>
          }
          confirmSummary={
            <>
              <strong>Close {tableLabel(closeTarget)} for good?</strong>
              <br />
              {payments[closeTarget.id]
                ? `${formatCurrency(payments[closeTarget.id].amount)} is still unpaid and will be cancelled.`
                : "No payment is outstanding on this table."}
              {" "}Guests lose access straight away and cannot rejoin this session.
            </>
          }
          submitLabel="Force-close table"
          onClose={() => setCloseTarget(null)}
          onSubmit={(reason) => handleForceClose(closeTarget, reason)}
        />
      )}
    </div>
  )
}

function tableLabel(s: Session): string {
  return s.table_identifier ?? `Table ${s.table_id}`
}

const rowActionStyle: React.CSSProperties = {
  height: 34, paddingInline: 14,
  fontSize: 12, fontWeight: 600,
  borderRadius: "var(--rad-md)",
  border: "1px solid",
  background: "transparent",
  cursor: "pointer",
}
