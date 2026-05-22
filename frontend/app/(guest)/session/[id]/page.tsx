"use client"

import Link from "next/link"
import { use } from "react"
import { useSession } from "@/hooks/useSession"
import { UtensilsCrossed, ClipboardList, Bell, CreditCard } from "lucide-react"

interface Props {
  params: Promise<{ id: string }>
}

function ParticipantStrip({ participants }: { participants: { id: number; display_name: string }[] }) {
  const visible = participants.slice(0, 5)
  const overflow = participants.length - 5

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {visible.map((p) => (
        <div
          key={p.id}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium"
          style={{
            backgroundColor: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            color: "var(--color-text)",
          }}
        >
          <span
            className="size-5 rounded-full flex items-center justify-center text-xs font-semibold"
            style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
            aria-hidden
          >
            {p.display_name[0]?.toUpperCase()}
          </span>
          {p.display_name}
        </div>
      ))}
      {overflow > 0 && (
        <span className="text-xs" style={{ color: "var(--color-text-muted)" }}>
          +{overflow} more
        </span>
      )}
    </div>
  )
}

export default function SessionLandingPage({ params }: Props) {
  const { id } = use(params)
  const { session, participant, participants } = useSession()

  const actions = [
    { href: `/session/${id}/menu`, label: "View Menu", description: "Browse what's available", icon: UtensilsCrossed },
    { href: `/session/${id}/orders`, label: "Orders", description: "Track your orders live", icon: ClipboardList },
    { href: `/session/${id}/assist`, label: "Need Help", description: "Call a waiter or request bill", icon: Bell },
    { href: `/session/${id}/payment`, label: "Pay Bill", description: "Settle up when ready", icon: CreditCard },
  ]

  return (
    <div
      className="px-4 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="space-y-1">
        <h1 className="text-xl font-semibold">
          {participant ? `Welcome, ${participant.display_name}` : "Your table"}
        </h1>
        {session && (
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            {session.table_identifier ?? `Table ${session.table_id}`}
          </p>
        )}
      </div>

      {participants.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
            At this table
          </p>
          <ParticipantStrip participants={participants} />
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        {actions.map(({ href, label, description, icon: Icon }) => (
          <Link
            key={href}
            href={href}
            className="flex flex-col gap-3 p-4 rounded-2xl transition-opacity active:opacity-70"
            style={{
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              boxShadow: "var(--shadow-card)",
              borderRadius: "var(--radius-lg)",
            }}
          >
            <div
              className="size-10 rounded-xl flex items-center justify-center"
              style={{ backgroundColor: "var(--color-bg)" }}
            >
              <Icon className="size-5" style={{ color: "var(--color-accent)" }} aria-hidden />
            </div>
            <div>
              <p className="font-semibold text-sm">{label}</p>
              <p className="text-xs mt-0.5" style={{ color: "var(--color-text-muted)" }}>
                {description}
              </p>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
