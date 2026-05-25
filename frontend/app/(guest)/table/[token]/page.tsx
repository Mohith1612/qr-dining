"use client"

import { useState, useEffect, use } from "react"
import { useRouter } from "next/navigation"
import { menuApi } from "@/lib/api/menu"
import { sessionsApi } from "@/lib/api/sessions"
import { useSessionStore } from "@/store/session"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
      <div
        className="min-h-svh flex items-center justify-center px-6"
        style={{ backgroundColor: "var(--color-bg)" }}
      >
        <div className="flex flex-col items-center gap-4">
          <div
            className="size-16 rounded-2xl flex items-center justify-center animate-pulse"
            style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
          />
          <div className="space-y-2 text-center">
            <div
              className="h-7 w-36 rounded-lg mx-auto animate-pulse"
              style={{ backgroundColor: "var(--color-surface)" }}
            />
            <div
              className="h-4 w-52 rounded-lg mx-auto animate-pulse"
              style={{ backgroundColor: "var(--color-surface)" }}
            />
          </div>
        </div>
      </div>
    )
  }

  if (error && !tableInfo) {
    return (
      <div
        className="min-h-svh flex flex-col items-center justify-center px-6 text-center gap-5"
        style={{ backgroundColor: "var(--color-bg)" }}
      >
        <div
          className="size-16 rounded-2xl flex items-center justify-center"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
        >
          <UtensilsCrossed className="size-7" style={{ color: "var(--color-text-muted)" }} aria-hidden />
        </div>
        <div className="space-y-2">
          <h1
            className="text-2xl font-medium"
            style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
          >
            Invalid code
          </h1>
          <p className="text-sm max-w-xs" style={{ color: "var(--color-text-muted)" }}>{error}</p>
        </div>
      </div>
    )
  }

  return (
    <div
      className="min-h-svh flex flex-col items-center justify-center px-6"
      style={{ backgroundColor: "var(--color-bg)" }}
    >
      <div className="w-full max-w-sm space-y-10">
        {/* Brand mark area */}
        <div className="text-center space-y-4">
          <div
            className="size-20 rounded-3xl flex items-center justify-center mx-auto"
            style={{
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              boxShadow: "var(--shadow-elevated)",
            }}
          >
            <UtensilsCrossed className="size-9" style={{ color: "var(--color-accent)" }} aria-hidden />
          </div>
          <div className="space-y-1.5">
            <h1
              className="text-3xl font-medium tracking-tight"
              style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
            >
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
            className="h-13 text-base rounded-xl text-center"
            style={{
              borderColor: "var(--color-border)",
              backgroundColor: "var(--color-surface)",
              color: "var(--color-text)",
              height: "52px",
              fontSize: "16px",
            }}
          />
          {error && (
            <p className="text-xs text-center" style={{ color: "var(--color-error)" }}>{error}</p>
          )}
          <Button
            type="submit"
            disabled={!name.trim() || loading}
            className="w-full rounded-xl font-medium"
            style={{
              backgroundColor: "var(--color-accent)",
              color: "var(--color-accent-fg)",
              height: "52px",
              fontSize: "15px",
            }}
          >
            {loading ? "Joining…" : "Join table"}
          </Button>
        </form>
      </div>
    </div>
  )
}
