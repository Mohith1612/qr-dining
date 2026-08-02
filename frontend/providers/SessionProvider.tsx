"use client"

import { ReactNode, useEffect, useRef, useState } from "react"
import { useWebSocket } from "@/hooks/useWebSocket"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { sessionsApi } from "@/lib/api/sessions"
import { ApiError } from "@/lib/api/client"
import { themeApi } from "@/lib/api/theme"
import { applyTheme } from "@/lib/theme/applyTheme"
import { useSessionStore } from "@/store/session"
import { useBrandingStore } from "@/store/branding"
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
  const { retry } = useWebSocket(sessionId)
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
      .then(r => {
        applyTheme(r.theme)
        useBrandingStore.getState().setBranding({ logoUrl: r.logo_url, name: r.restaurant_name })
      })
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
    }).catch(async (err) => {
      // A closed session invalidates the guest token (snapshot → 401) and, once the
      // terminal read window lapses, returns 410 SESSION_ENDED. In both cases a stale
      // tab should land on the ended screen instead of looping on reconnect. The
      // snapshot still exposes terminal status without a token (when guest creds
      // aren't required), so retry once without it to detect a closed session.
      if (err instanceof ApiError && (err.status === 410 || err.code === "SESSION_ENDED")) {
        if (!cancelled) setSessionClosed(true)
      } else {
        try {
          const snap = await sessionsApi.snapshot(sessionId)
          if (!cancelled && TERMINAL_STATUSES.includes(snap.session.status)) setSessionClosed(true)
        } catch { /* session genuinely unreachable */ }
      }
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

  return (
    <ErrorBoundary>
      {isReactivating ? <SessionReactivatingBanner /> : <ReconnectingBanner onRetry={retry} />}
      {snapshotLoaded ? children : null}
      <SessionTimeoutBanner />
    </ErrorBoundary>
  )
}
