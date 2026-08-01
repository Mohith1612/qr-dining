"use client"

import { use } from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { SessionProvider } from "@/providers/SessionProvider"
import { Shell } from "@/components/layout/Shell"
import { BottomNav } from "@/components/layout/BottomNav"
import { TopBar } from "@/components/layout/TopBar"
import { ErrorBoundary } from "@/providers/ErrorBoundary"
import { recoverGuestCreds } from "@/lib/guest-session"

interface Props {
  children: React.ReactNode
  params: Promise<{ id: string }>
}

export default function SessionLayout({ children, params }: Props) {
  const { id } = use(params)
  const router = useRouter()
  const [participantId, setParticipantId] = useState<number | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    // Prefer this tab's sessionStorage; fall back to the localStorage rejoin
    // copy so reopening a /session/<id> link in a fresh tab recovers the
    // session instead of bouncing to the landing page.
    const creds = recoverGuestCreds(id)
    if (!creds) {
      router.replace(`/`)
      return
    }

    setParticipantId(creds.participantId)
    setReady(true)
  }, [id, router])

  if (!ready || participantId === null) return null

  return (
    <ErrorBoundary>
      <SessionProvider sessionId={id} participantId={participantId}>
        <Shell
          topBar={<TopBar />}
          nav={<BottomNav sessionId={id} />}
        >
          {children}
        </Shell>
      </SessionProvider>
    </ErrorBoundary>
  )
}
