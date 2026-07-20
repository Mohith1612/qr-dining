"use client"

import Link from "next/link"
import { use } from "react"
import { toast } from "sonner"
import { useSession } from "@/hooks/useSession"
import { sessionsApi } from "@/lib/api/sessions"
import { ApiError } from "@/lib/api/client"
import { Avatar } from "@/components/shared/Avatar"
import type { Participant } from "@/types/api"
import { UtensilsCrossed, ClipboardList, Bell, CreditCard, ChevronRight, Crown } from "lucide-react"

interface Props {
  params: Promise<{ id: string }>
}

const TILE_TONES: Record<string, { bg: string; fg: string }> = {
  accent:  { bg: "var(--accent-soft)",  fg: "var(--accent)"  },
  info:    { bg: "var(--info-soft)",    fg: "var(--info)"    },
  warn:    { bg: "var(--warn-soft)",    fg: "var(--warn)"    },
  neutral: { bg: "var(--line-1)",       fg: "var(--ink-2)"   },
}

export default function SessionLandingPage({ params }: Props) {
  const { id } = use(params)
  const { session, participant, participants, isHost } = useSession()

  // Host only: hand the host role to another participant. HOST_CHANGED (WS)
  // updates every client's badges and host-only controls.
  async function makeHost(target: Participant) {
    if (!session) return
    if (!window.confirm(`Make ${target.display_name} the host? They'll be able to send orders and pay the bill.`)) return
    const guestToken = sessionStorage.getItem("guest_access_token") ?? ""
    try {
      await sessionsApi.transferHost(session.id, target.id, guestToken)
      toast.success(`${target.display_name} is now the host.`)
    } catch (e) {
      const code = e instanceof ApiError ? e.code : ""
      toast.error(
        code === "HOST_TRANSFER_LOCKED" ? "You can't change host while a payment is in progress."
        : code === "NOT_SESSION_HOST" ? "Only the current host can do that."
        : code === "VALIDATION_ERROR" ? "That guest is no longer at the table."
        : "Couldn't transfer host. Please try again."
      )
    }
  }

  const actions = [
    { href: `/session/${id}/menu`,    label: "View Menu",  sub: "Tonight's offerings",            icon: UtensilsCrossed, tone: "accent"  },
    { href: `/session/${id}/orders`,  label: "Orders",     sub: "Track live from the kitchen",    icon: ClipboardList,   tone: "info"    },
    { href: `/session/${id}/assist`,  label: "Need Help",  sub: "Call a host, or anything else",  icon: Bell,            tone: "warn"    },
    { href: `/session/${id}/payment`, label: "Pay Bill",   sub: "Settle when you're ready",       icon: CreditCard,      tone: "neutral" },
  ]

  const tableLabel = session ? (session.table_identifier ?? `Table ${session.table_id}`) : "Table"

  return (
    <div className="screen-enter px-5 py-6 pb-8" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Greeting */}
      <div style={{ marginBottom: 24 }}>
        <p className="eyebrow">Good evening</p>
        <h1 className="serif" style={{ fontSize: 34, fontWeight: 500, letterSpacing: "-0.02em", color: "var(--ink-1)", lineHeight: 1.1, margin: "6px 0 0" }}>
          {participant ? `Welcome, ${participant.display_name}.` : "Welcome."}
        </h1>
        <p style={{ color: "var(--ink-2)", fontSize: 14, marginTop: 4 }}>Seated at {tableLabel}</p>
      </div>

      {/* Dining party card */}
      <div style={{
        background: "linear-gradient(180deg, var(--bg-elev-2), var(--bg-elev-1))",
        border: "1px solid var(--line-2)",
        borderRadius: "var(--rad-lg)",
        boxShadow: "var(--shadow-2)",
        padding: 16,
        marginBottom: 24,
      }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
          <p className="eyebrow">At this table</p>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 11.5 }}>
            <span className="live-dot" />
            in sync
          </span>
        </div>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
          {participants.slice(0, 6).map((p) => {
            const isYou = p.id === participant?.id
            return (
              <div key={p.id} style={{ display: "inline-flex", alignItems: "center", gap: 7, padding: "5px 12px 5px 5px", borderRadius: 999, background: "var(--bg-elev-3)", border: "1px solid var(--line-1)" }}>
                <Avatar name={p.display_name} size={24} />
                <span style={{ fontSize: 12.5, color: "var(--ink-1)", fontWeight: 500 }}>{p.display_name}</span>
                {p.is_host && (
                  <span
                    title="Session host — sends orders and settles the bill"
                    style={{ fontSize: 9.5, color: "var(--accent-ink)", background: "var(--accent)", padding: "2px 6px", borderRadius: 999, letterSpacing: "0.06em", fontWeight: 700, textTransform: "uppercase" }}
                  >
                    Host
                  </span>
                )}
                {isYou && <span style={{ fontSize: 10, color: "var(--accent)", letterSpacing: "0.08em", fontWeight: 600 }}>YOU</span>}
                {isHost && !p.is_host && !isYou && (
                  <button
                    onClick={() => makeHost(p)}
                    title={`Make ${p.display_name} the host`}
                    aria-label={`Make ${p.display_name} the host`}
                    className="press"
                    style={{
                      display: "inline-flex", alignItems: "center", gap: 4,
                      marginLeft: 2, padding: "3px 8px", borderRadius: 999,
                      background: "transparent", border: "1px solid var(--line-2)",
                      color: "var(--ink-3)", fontSize: 10.5, fontWeight: 600,
                      letterSpacing: "0.04em", cursor: "pointer",
                    }}
                  >
                    <Crown size={11} aria-hidden />
                    Make host
                  </button>
                )}
              </div>
            )
          })}
          {participants.length > 6 && <span style={{ fontSize: 12, color: "var(--ink-3)" }}>+{participants.length - 6} more</span>}
        </div>
      </div>

      {/* Action tiles 2×2 */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        {actions.map(({ href, label, sub, icon: Icon, tone }) => {
          const colors = TILE_TONES[tone]
          return (
            <Link
              key={href}
              href={href}
              className="press"
              style={{
                display: "flex", flexDirection: "column", gap: 12,
                padding: "16px 14px",
                borderRadius: "var(--rad-lg)",
                background: "var(--bg-elev-1)",
                border: "1px solid var(--line-1)",
                boxShadow: "var(--shadow-1)",
                minHeight: 110,
                textDecoration: "none",
              }}
            >
              <span style={{ width: 36, height: 36, borderRadius: 12, background: colors.bg, color: colors.fg, display: "inline-flex", alignItems: "center", justifyContent: "center" }}>
                <Icon size={18} />
              </span>
              <div>
                <div style={{ fontSize: 15, fontWeight: 600, color: "var(--ink-1)", letterSpacing: "-0.005em" }}>{label}</div>
                <div style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 2, lineHeight: 1.4 }}>{sub}</div>
              </div>
            </Link>
          )
        })}
      </div>

      {/* Footer brand */}
      <div style={{ marginTop: 36, display: "flex", alignItems: "center", justifyContent: "center", gap: 6, color: "var(--ink-4)", fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase" }}>
        <UtensilsCrossed size={11} aria-hidden />
        QR Dining
      </div>
    </div>
  )
}
