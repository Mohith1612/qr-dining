"use client"

import { use } from "react"
import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { SessionProvider } from "@/providers/SessionProvider"
import { Shell } from "@/components/layout/Shell"
import { BottomNav } from "@/components/layout/BottomNav"
import { TopBar } from "@/components/layout/TopBar"
import { ErrorBoundary } from "@/providers/ErrorBoundary"

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
    const storedSession = sessionStorage.getItem("session_id")
    const storedParticipant = sessionStorage.getItem("participant_id")

    if (!storedSession || storedSession !== id || !storedParticipant) {
      router.replace(`/`)
      return
    }

    setParticipantId(Number(storedParticipant))
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
