"use client"

import { ReactNode, useEffect, useRef, useState } from "react"
import { useWebSocket } from "@/hooks/useWebSocket"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { sessionsApi } from "@/lib/api/sessions"
import { themeApi } from "@/lib/api/theme"
import { applyTheme } from "@/lib/theme/applyTheme"
import { useSessionStore } from "@/store/session"
import { ReconnectingBanner } from "@/components/shared/ReconnectingBanner"
import { SessionEndedScreen } from "@/components/shared/SessionEndedScreen"
import { SessionReactivatingBanner } from "@/components/shared/SessionReactivatingBanner"
import { SessionTimeoutBanner } from "@/components/shared/SessionTimeoutBanner"
import { ErrorBoundary } from "./ErrorBoundary"

interface SessionProviderProps {
  sessionId: string
  participantId: number
  children: ReactNode
}

const TERMINAL_STATUSES: string[] = ["closed", "abandoned", "expired"]

export function SessionProvider({ sessionId, participantId, children }: SessionProviderProps) {
  const [snapshotLoaded, setSnapshotLoaded] = useState(false)
  const [sessionClosed, setSessionClosed] = useState(false)
  const { status } = useWebSocket(sessionId)
  const session = useSessionStore((s) => s.session)
  const isReactivating = useSessionStore((s) => s.isReactivating)
  const completedPayment = useSessionStore((s) => s.completedPayment)
  const themedBranchRef = useRef<number | null>(null)

  // Apply the branch's structured theme once per branch id (covers reloads, fresh
  // sessions, and reactivation; re-applies on a branch change without refetching
  // on every render).
  const branchId = session?.branch_id ?? null
  useEffect(() => {
    if (!branchId || themedBranchRef.current === branchId) return
    themedBranchRef.current = branchId
    themeApi.resolveForBranch(branchId)
      .then(r => applyTheme(r.theme))
      .catch(() => {})
  }, [branchId])

  useEffect(() => {
    let cancelled = false
    const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
    sessionsApi.snapshot(sessionId, guestToken).then((snap) => {
      if (cancelled) return
      reconcileSnapshot(snap)
      // If participant wasn't set by setSession (e.g. after a hard navigation),
      // find self in the snapshot using the participantId from sessionStorage.
      if (!useSessionStore.getState().participant) {
        const self = snap.participants.find((p) => p.id === participantId)
        if (self) {
          const session = snap.table_identifier
            ? { ...snap.session, table_identifier: snap.table_identifier }
            : snap.session
          useSessionStore.getState().setSession(session, self)
        }
      }
      if (TERMINAL_STATUSES.includes(snap.session.status)) {
        setSessionClosed(true)
      }
      setSnapshotLoaded(true)
    }).catch(() => {
      if (!cancelled) setSnapshotLoaded(true)
    })
    return () => { cancelled = true }
  }, [sessionId, participantId])

  useEffect(() => {
    if (session?.status && TERMINAL_STATUSES.includes(session.status)) {
      setSessionClosed(true)
    }
  }, [session?.status])

  if (sessionClosed) return (
    <SessionEndedScreen
      paymentStatus={completedPayment ? "completed" : null}
      totalAmount={completedPayment ? parseFloat(completedPayment.amount) : undefined}
    />
  )

  if (status === "failed") return (
    <div style={{
      minHeight: "100svh", display: "flex", flexDirection: "column",
      alignItems: "center", justifyContent: "center",
      padding: 32, textAlign: "center", background: "var(--bg-base)",
    }}>
      <p style={{ fontSize: 20, fontWeight: 600, color: "var(--ink-1)", marginBottom: 8 }}>
        Connection lost
      </p>
      <p style={{ fontSize: 14, color: "var(--ink-3)", marginBottom: 24 }}>
        We couldn&apos;t reconnect to the server.
      </p>
      <button
        onClick={() => window.location.reload()}
        style={{
          padding: "10px 24px", borderRadius: 999,
          background: "var(--accent)", color: "var(--accent-ink)",
          fontSize: 14, fontWeight: 600, cursor: "pointer", border: "none",
        }}
      >
        Refresh page
      </button>
    </div>
  )

  return (
    <ErrorBoundary>
      {isReactivating ? <SessionReactivatingBanner /> : <ReconnectingBanner />}
      {snapshotLoaded ? children : null}
      <SessionTimeoutBanner />
    </ErrorBoundary>
  )
}
