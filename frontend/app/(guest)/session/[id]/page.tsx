"use client"

import Link from "next/link"
import { use, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { useSession } from "@/hooks/useSession"
import { sessionsApi } from "@/lib/api/sessions"
import { menuApi } from "@/lib/api/menu"
import { ApiError } from "@/lib/api/client"
import { Avatar } from "@/components/shared/Avatar"
import { FeaturedCarousel } from "@/components/shared/FeaturedCarousel"
import { useOrdersStore } from "@/store/orders"
import type { Participant, MenuItem } from "@/types/api"
import { UtensilsCrossed, ClipboardList, Bell, Receipt, ChevronRight, Crown } from "lucide-react"

interface Props {
  params: Promise<{ id: string }>
}

export default function SessionLandingPage({ params }: Props) {
  const { id } = use(params)
  const router = useRouter()
  const { session, participant, participants, isHost } = useSession()
  const hasOrders = useOrdersStore((s) => s.orders.length > 0)
  const [specials, setSpecials] = useState<MenuItem[]>([])
  const [popular, setPopular] = useState<MenuItem[]>([])

  // Pull the menu once so the landing can surface tonight's specials + popular
  // dishes. Tapping any of them deep-links into the menu (item sheet wired in A5).
  useEffect(() => {
    const branchId = session?.branch_id
    if (!branchId) return
    let active = true
    menuApi.getMenu(branchId)
      .then(({ featured, categories }) => {
        if (!active) return
        const all = categories.flatMap((c) => c.items).filter((i) => i.is_available)
        const feat = featured.filter((i) => i.is_available)
        const featIds = new Set(feat.map((i) => i.id))
        const bestsellers = all.filter((i) => i.item_badges?.includes("bestseller") && !featIds.has(i.id))
        const pool = bestsellers.length ? bestsellers : all.filter((i) => !featIds.has(i.id))
        setSpecials(feat)
        setPopular(pool.slice(0, 8))
      })
      .catch(() => { /* landing degrades gracefully without specials */ })
    return () => { active = false }
  }, [session?.branch_id])

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

  // Shown only after the party's first order — before that the landing stays
  // simple: menu + a way to call a waiter.
  const secondary = [
    { href: `/session/${id}/orders`,  label: "Orders", sub: "Track the kitchen", icon: ClipboardList },
    { href: `/session/${id}/assist`,  label: "Help",   sub: "Call your host",    icon: Bell          },
    { href: `/session/${id}/payment`, label: "Bill",   sub: "Settle up",         icon: Receipt       },
  ]

  const tableLabel = session ? (session.table_identifier ?? `Table ${session.table_id}`) : "Table"
  const goToItem = (item: MenuItem) => router.push(`/session/${id}/menu?item=${item.id}`)

  return (
    <div className="screen-enter px-5 py-6 pb-10" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Greeting */}
      <div style={{ marginBottom: 22 }}>
        <p className="eyebrow">Good evening</p>
        <h1 className="serif" style={{ fontSize: "clamp(28px, 8vw, 36px)", fontWeight: 600, letterSpacing: "-0.02em", color: "var(--ink-1)", lineHeight: 1.1, margin: "6px 0 0" }}>
          {participant ? `Welcome, ${participant.display_name}.` : "Welcome."}
        </h1>
        <p style={{ color: "var(--ink-2)", fontSize: 15, marginTop: 6 }}>Seated at {tableLabel}</p>
      </div>

      {/* Dining party card */}
      <div style={{
        background: "var(--bg-elev-1)",
        border: "1px solid var(--line-1)",
        borderRadius: "var(--rad-lg)",
        boxShadow: "var(--shadow-1)",
        padding: 16,
        marginBottom: 28,
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
              <div key={p.id} style={{ display: "inline-flex", alignItems: "center", gap: 7, padding: "5px 12px 5px 5px", borderRadius: 999, background: "var(--bg-sunken)", border: "1px solid var(--line-1)" }}>
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

      {/* Currently Popular — horizontal carousel */}
      {popular.length > 0 && (
        <div style={{ margin: "0 -20px 24px" }}>
          <div style={{ padding: "0 20px 2px" }}>
            <p className="eyebrow">Currently Popular</p>
          </div>
          <FeaturedCarousel items={popular} onSelect={goToItem} heading={null} cardEyebrow="Guest Favourite" />
        </div>
      )}

      {hasOrders ? (
        <>
          {/* Primary action — View Menu */}
          <Link
            href={`/session/${id}/menu`}
            className="press"
            style={{
              display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12,
              padding: "18px 20px", marginBottom: 12,
              borderRadius: "var(--rad-lg)",
              background: "var(--accent)", color: "var(--accent-ink)",
              boxShadow: "var(--shadow-2)", textDecoration: "none",
            }}
          >
            <span style={{ display: "flex", alignItems: "center", gap: 14 }}>
              <span style={{ width: 42, height: 42, borderRadius: "var(--rad-md)", background: "rgba(255,255,255,0.12)", display: "inline-flex", alignItems: "center", justifyContent: "center" }}>
                <UtensilsCrossed size={20} />
              </span>
              <span style={{ display: "flex", flexDirection: "column" }}>
                <span style={{ fontSize: 17, fontWeight: 600, letterSpacing: "-0.01em" }}>View Menu</span>
                <span style={{ fontSize: 12.5, color: "rgba(255,255,255,0.72)", marginTop: 1 }}>Tonight&apos;s offerings</span>
              </span>
            </span>
            <ChevronRight size={20} style={{ opacity: 0.8 }} aria-hidden />
          </Link>

          {/* Secondary actions */}
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 10, marginBottom: 24 }}>
            {secondary.map(({ href, label, sub, icon: Icon }) => (
              <Link
                key={href}
                href={href}
                className="press"
                style={{
                  display: "flex", flexDirection: "column", gap: 10,
                  padding: "14px 12px",
                  borderRadius: "var(--rad-lg)",
                  background: "var(--bg-elev-1)",
                  border: "1px solid var(--line-1)",
                  boxShadow: "var(--shadow-1)",
                  textDecoration: "none",
                }}
              >
                <span style={{ width: 34, height: 34, borderRadius: "var(--rad-sm)", background: "var(--accent-soft)", color: "var(--ink-1)", display: "inline-flex", alignItems: "center", justifyContent: "center" }}>
                  <Icon size={17} />
                </span>
                <span>
                  <div style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{label}</div>
                  <div style={{ fontSize: 11.5, color: "var(--ink-3)", marginTop: 1, lineHeight: 1.35 }}>{sub}</div>
                </span>
              </Link>
            ))}
          </div>
        </>
      ) : (
        /* First landing — Menu + Help side by side; Orders/Bill arrive with the first order */
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10, marginBottom: 24 }}>
          <Link
            href={`/session/${id}/menu`}
            className="press"
            style={{
              display: "flex", flexDirection: "column", gap: 10,
              padding: "16px 14px",
              borderRadius: "var(--rad-lg)",
              background: "var(--accent)", color: "var(--accent-ink)",
              boxShadow: "var(--shadow-2)", textDecoration: "none",
            }}
          >
            <span style={{ width: 34, height: 34, borderRadius: "var(--rad-sm)", background: "rgba(255,255,255,0.12)", display: "inline-flex", alignItems: "center", justifyContent: "center" }}>
              <UtensilsCrossed size={17} />
            </span>
            <span>
              <div style={{ fontSize: 14, fontWeight: 600 }}>View Menu</div>
              <div style={{ fontSize: 11.5, opacity: 0.75, marginTop: 1, lineHeight: 1.35 }}>Tonight&apos;s offerings</div>
            </span>
          </Link>
          <Link
            href={`/session/${id}/assist`}
            className="press"
            style={{
              display: "flex", flexDirection: "column", gap: 10,
              padding: "16px 14px",
              borderRadius: "var(--rad-lg)",
              background: "var(--bg-elev-1)",
              border: "1px solid var(--line-1)",
              boxShadow: "var(--shadow-1)",
              textDecoration: "none",
            }}
          >
            <span style={{ width: 34, height: 34, borderRadius: "var(--rad-sm)", background: "var(--accent-soft)", color: "var(--ink-1)", display: "inline-flex", alignItems: "center", justifyContent: "center" }}>
              <Bell size={17} />
            </span>
            <span>
              <div style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>Help</div>
              <div style={{ fontSize: 11.5, color: "var(--ink-3)", marginTop: 1, lineHeight: 1.35 }}>Call your host</div>
            </span>
          </Link>
        </div>
      )}

      {/* Today's Specials — reuses the featured carousel */}
      {specials.length > 0 && (
        <div style={{ margin: "0 -20px 28px" }}>
          <div style={{ padding: "0 20px 2px" }}>
            <p className="eyebrow" style={{ color: "var(--accent)" }}>Today&apos;s Specials</p>
          </div>
          <FeaturedCarousel items={specials} onSelect={goToItem} heading={null} />
        </div>
      )}

      {/* Footer brand */}
      <div style={{ marginTop: 8, display: "flex", alignItems: "center", justifyContent: "center", gap: 6, color: "var(--ink-4)", fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase" }}>
        <UtensilsCrossed size={11} aria-hidden />
        QR Dining
      </div>
    </div>
  )
}
