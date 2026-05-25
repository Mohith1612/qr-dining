"use client"

import { useState, useEffect, use } from "react"
import { useRouter } from "next/navigation"
import { menuApi } from "@/lib/api/menu"
import { sessionsApi } from "@/lib/api/sessions"
import { useSessionStore } from "@/store/session"
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
      <div className="atmos min-h-svh flex items-center justify-center" style={{ background: "var(--bg-base)" }}>
        <div style={{ position: "absolute", inset: 0, background: "var(--glow-warm)", pointerEvents: "none" }} />
        <div className="relative z-10 flex flex-col items-center gap-4">
          <div className="skeleton" style={{ width: 88, height: 88, borderRadius: 26 }} />
          <div className="skeleton" style={{ height: 28, width: 160, borderRadius: 8 }} />
          <div className="skeleton" style={{ height: 16, width: 220, borderRadius: 8 }} />
        </div>
      </div>
    )
  }

  if (error && !tableInfo) {
    return (
      <div className="atmos min-h-svh flex flex-col items-center justify-center px-8 text-center gap-5" style={{ background: "var(--bg-base)" }}>
        <div style={{ position: "absolute", inset: 0, background: "var(--glow-warm)", pointerEvents: "none" }} />
        <div className="relative z-10 flex flex-col items-center gap-4">
          <div style={{ width: 64, height: 64, borderRadius: "var(--rad-lg)", background: "var(--bg-elev-2)", border: "1px solid var(--line-2)", boxShadow: "var(--shadow-2)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <UtensilsCrossed size={28} style={{ color: "var(--ink-3)" }} aria-hidden />
          </div>
          <h1 className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--ink-1)" }}>Invalid code</h1>
          <p style={{ color: "var(--ink-2)", fontSize: 14, maxWidth: 280 }}>{error}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="atmos min-h-svh flex flex-col items-center justify-center screen-enter" style={{ background: "var(--bg-base)" }}>
      <div style={{ position: "absolute", inset: 0, background: "var(--glow-warm)", pointerEvents: "none" }} />
      <div className="relative z-10 w-full px-7" style={{ maxWidth: 420 }}>
        {/* Heading */}
        <div style={{ textAlign: "center", marginBottom: 28 }}>
          <p className="eyebrow">Table {tableInfo?.label ?? tableInfo?.table_id}</p>
          <h1 className="serif" style={{ margin: "10px 0 6px", fontSize: 38, fontWeight: 500, letterSpacing: "-0.02em", color: "var(--ink-1)", lineHeight: 1.05 }}>
            Welcome.
          </h1>
          <p style={{ color: "var(--ink-2)", fontSize: 14.5, lineHeight: 1.55, maxWidth: 260, marginInline: "auto" }}>
            What should we call you? Your party will see your name on shared orders.
          </p>
        </div>

        {/* Card form */}
        <div style={{ background: "var(--bg-elev-2)", border: "1px solid var(--line-2)", borderRadius: "var(--rad-lg)", boxShadow: "var(--shadow-2)", padding: "18px 18px 16px" }}>
          <label className="eyebrow" style={{ display: "block", marginBottom: 8 }}>Your name</label>
          <form onSubmit={handleSubmit}>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Aanya"
              maxLength={40}
              autoFocus
              required
              style={{
                width: "100%", border: 0, outline: 0, background: "transparent",
                fontFamily: "var(--font-display, 'Cormorant Garamond', Georgia, serif)",
                fontSize: 22, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.01em",
              }}
            />
            <hr className="rule" style={{ marginTop: 6, marginBottom: 14 }} />
            {error && <p style={{ color: "var(--alert)", fontSize: 12, marginBottom: 10, textAlign: "center" }}>{error}</p>}
            <button
              type="submit"
              disabled={!name.trim() || loading}
              className="press"
              style={{
                width: "100%", height: 52, borderRadius: "var(--rad-md)",
                background: "var(--accent)", color: "var(--accent-ink)",
                border: 0, fontSize: 15, fontWeight: 600, cursor: loading ? "not-allowed" : "pointer",
                opacity: !name.trim() || loading ? 0.6 : 1,
              }}
            >
              {loading ? "Joining…" : "Join the table →"}
            </button>
          </form>
        </div>

        {/* Footer */}
        <div style={{ marginTop: 22, display: "flex", alignItems: "center", justifyContent: "center", gap: 8, color: "var(--ink-3)", fontSize: 12 }}>
          <span className="live-dot" />
          Live · joining as guest
        </div>
      </div>
    </div>
  )
}
