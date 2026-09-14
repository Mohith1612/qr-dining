"use client"

import { ReactNode, useCallback, useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { useWebSocket } from "@/hooks/useWebSocket"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { sessionsApi } from "@/lib/api/sessions"
import { ApiError, isRevokedCredentialError } from "@/lib/api/client"
import { clearGuestCreds } from "@/lib/guest-session"
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
  const router = useRouter()
  const { retry } = useWebSocket(sessionId)
  const session = useSessionStore((s) => s.session)
  const isReactivating = useSessionStore((s) => s.isReactivating)
  const completedPayment = useSessionStore((s) => s.completedPayment)
  const themedBranchRef = useRef<number | null>(null)

  // The single place the session becomes terminal for this client. Every
  // terminal path funnels through here — snapshot reports a terminal status, the
  // read window lapsed (410), the credential was revoked (401), or a
  // SESSION_CLOSED event flipped the store — so it is also the one place that
  // has to forget the stored credential (F-13). The guest bearer is good for 12
  // hours and nothing else ever cleared it.
  const endSession = useCallback(() => {
    clearGuestCreds(sessionId)
    setSessionClosed(true)
  }, [sessionId])

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
        endSession()
      }
      setSnapshotLoaded(true)
    }).catch(async (err) => {
      // A closed session ends a stale tab in one of two ways. Once the terminal
      // read window lapses the snapshot returns 410 SESSION_ENDED. Before that,
      // the close has already rotated every participant's credential_version, so
      // the stored token fails auth and the snapshot returns 401 — which is the
      // case this used to miss entirely, leaving the tab on "Reconnecting…"
      // forever (L-09 / F-22).
      //
      // 401 is not one thing, so we do not treat it as one: the server flags the
      // unrecoverable ones with reason `credential_revoked`. Those are terminal
      // and get the ended screen. Any other 401 (no credential, expired or
      // malformed token) means we cannot prove membership of a session that may
      // well still be running — the honest move is to forget the credential and
      // send the guest back to rescan, not to claim their meal is over.
      if (cancelled) return
      if (isRevokedCredentialError(err)) {
        endSession()
      } else if (err instanceof ApiError && (err.status === 410 || err.code === "SESSION_ENDED")) {
        endSession()
      } else if (err instanceof ApiError && err.status === 401) {
        clearGuestCreds(sessionId)
        router.replace("/")
        return
      } else {
        // Not an auth failure. The snapshot still exposes terminal status
        // without a token (when guest creds aren't required), so retry once
        // without it to detect a closed session.
        try {
          const snap = await sessionsApi.snapshot(sessionId)
          if (!cancelled && TERMINAL_STATUSES.includes(snap.session.status)) endSession()
        } catch { /* session genuinely unreachable */ }
      }
      if (!cancelled) setSnapshotLoaded(true)
    })
    return () => { cancelled = true }
  }, [sessionId, participantId, endSession, router])

  // Covers the live path: a SESSION_CLOSED event (or the host closing the table
  // from this device) flips the store status, which ends the session here and
  // clears the stored credential.
  useEffect(() => {
    if (session?.status && TERMINAL_STATUSES.includes(session.status)) {
      endSession()
    }
  }, [session?.status, endSession])

  if (sessionClosed) return (
    <SessionEndedScreen
      paymentStatus={completedPayment ? "completed" : null}
      totalAmount={completedPayment?.amount}
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
