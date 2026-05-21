"use client"

import { useState, useEffect, use } from "react"
import { useRouter } from "next/navigation"
import { menuApi } from "@/lib/api/menu"
import { sessionsApi } from "@/lib/api/sessions"
import { useSessionStore } from "@/store/session"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { UtensilsCrossed } from "lucide-react"

interface Props {
  params: Promise<{ token: string }>
}

export default function TableEntryPage({ params }: Props) {
  const { token } = use(params)
  const router = useRouter()
  const [tableInfo, setTableInfo] = useState<{ table_id: number; branch_id: number; label?: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [name, setName] = useState("")
  const [loading, setLoading] = useState(false)
  const [resolving, setResolving] = useState(true)
  const existingSession = typeof window !== "undefined" ? sessionStorage.getItem("session_id") : null

  useEffect(() => {
    menuApi.resolveQrToken(token)
      .then(setTableInfo)
      .catch(() => setError("This QR code is invalid or has expired."))
      .finally(() => setResolving(false))
  }, [token])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!tableInfo || !name.trim()) return
    setLoading(true)
    try {
      const { session, participant } = await sessionsApi.create(tableInfo.table_id, name.trim())
      useSessionStore.getState().setSession(session, participant)
      sessionStorage.setItem("session_id", session.id)
      sessionStorage.setItem("participant_id", String(participant.id))
      router.push(`/session/${session.id}`)
    } catch {
      setError("Something went wrong. Please try again.")
      setLoading(false)
    }
  }

  if (resolving) {
    return (
      <div className="min-h-svh flex items-center justify-center px-6">
        <div className="w-full max-w-sm space-y-4">
          <Skeleton className="h-16 w-16 rounded-2xl mx-auto" />
          <Skeleton className="h-7 w-48 mx-auto" />
          <Skeleton className="h-4 w-64 mx-auto" />
          <Skeleton className="h-12 w-full rounded-xl" />
          <Skeleton className="h-12 w-full rounded-xl" />
        </div>
      </div>
    )
  }

  if (error && !tableInfo) {
    return (
      <div
        className="min-h-svh flex flex-col items-center justify-center px-6 text-center gap-4"
        style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
      >
        <UtensilsCrossed className="size-10" style={{ color: "var(--color-text-muted)" }} aria-hidden />
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>{error}</p>
      </div>
    )
  }

  return (
    <div
      className="min-h-svh flex flex-col items-center justify-center px-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="w-full max-w-sm space-y-8">
        <div className="text-center space-y-3">
          <div
            className="size-16 rounded-2xl flex items-center justify-center mx-auto"
            style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
          >
            <UtensilsCrossed className="size-8" style={{ color: "var(--color-accent)" }} aria-hidden />
          </div>
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {tableInfo?.label ?? `Table ${tableInfo?.table_id}`}
            </h1>
            <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
              What should we call you?
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-3">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Your name"
            maxLength={40}
            autoFocus
            required
            className="h-12 text-base rounded-xl"
            style={{ borderColor: "var(--color-border)" }}
          />
          {error && (
            <p className="text-xs text-center" style={{ color: "var(--color-error)" }}>{error}</p>
          )}
          <Button
            type="submit"
            disabled={!name.trim() || loading}
            className="w-full h-12 rounded-xl text-base font-medium"
            style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
          >
            {loading ? "Joining…" : "Join table"}
          </Button>
        </form>
      </div>
    </div>
  )
}
