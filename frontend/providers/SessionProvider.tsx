"use client"

import { ReactNode, useEffect, useState } from "react"
import { useWebSocket } from "@/hooks/useWebSocket"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { sessionsApi } from "@/lib/api/sessions"
import { useSessionStore } from "@/store/session"
import { ReconnectingBanner } from "@/components/shared/ReconnectingBanner"
import { SessionEndedScreen } from "@/components/shared/SessionEndedScreen"
import { SessionTimeoutBanner } from "@/components/shared/SessionTimeoutBanner"
import { ErrorBoundary } from "./ErrorBoundary"

interface SessionProviderProps {
  sessionId: string
  participantId: number
  children: ReactNode
}

export function SessionProvider({ sessionId, participantId, children }: SessionProviderProps) {
  const [snapshotLoaded, setSnapshotLoaded] = useState(false)
  const [sessionClosed, setSessionClosed] = useState(false)
  const { status } = useWebSocket(sessionId, participantId)
  const session = useSessionStore((s) => s.session)
  const completedPayment = useSessionStore((s) => s.completedPayment)

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
      if (snap.session.status !== "active") {
        setSessionClosed(true)
      }
      setSnapshotLoaded(true)
    }).catch(() => {
      if (!cancelled) setSnapshotLoaded(true)
    })
    return () => { cancelled = true }
  }, [sessionId, participantId])

  useEffect(() => {
    if (session?.status === "closed" || session?.status === "abandoned") {
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
      <ReconnectingBanner />
      {snapshotLoaded ? children : null}
      <SessionTimeoutBanner />
    </ErrorBoundary>
  )
}
