"use client"

import { ReactNode, useEffect, useState } from "react"
import { useWebSocket } from "@/hooks/useWebSocket"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { sessionsApi } from "@/lib/api/sessions"
import { useSessionStore } from "@/store/session"
import { ReconnectingBanner } from "@/components/shared/ReconnectingBanner"
import { SessionEndedScreen } from "@/components/shared/SessionEndedScreen"
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

  useEffect(() => {
    let cancelled = false
    sessionsApi.snapshot(sessionId).then((snap) => {
      if (cancelled) return
      reconcileSnapshot(snap)
      if (snap.session.status !== "active") {
        setSessionClosed(true)
      }
      setSnapshotLoaded(true)
    }).catch(() => {
      if (!cancelled) setSnapshotLoaded(true)
    })
    return () => { cancelled = true }
  }, [sessionId])

  useEffect(() => {
    if (session?.status === "closed" || session?.status === "abandoned") {
      setSessionClosed(true)
    }
  }, [session?.status])

  if (sessionClosed) return <SessionEndedScreen />

  return (
    <ErrorBoundary>
      <ReconnectingBanner />
      {snapshotLoaded ? children : null}
    </ErrorBoundary>
  )
}
