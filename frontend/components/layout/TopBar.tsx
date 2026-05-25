"use client"

import { Users } from "lucide-react"
import { useSessionStore } from "@/store/session"
import { RealtimeIndicator } from "@/components/shared/RealtimeIndicator"

export function TopBar() {
  const session = useSessionStore((s) => s.session)
  const participants = useSessionStore((s) => s.participants)

  const tableLabel = session
    ? (session.table_identifier ?? `Table ${session.table_id}`)
    : "Table"

  return (
    <header
      className="flex items-center justify-between px-5 border-b sticky top-0 z-30"
      style={{
        backgroundColor: "var(--color-surface)",
        borderColor: "var(--color-border)",
        paddingTop: "calc(0.875rem + env(safe-area-inset-top))",
        paddingBottom: "0.875rem",
      }}
    >
      <span
        className="text-xs font-semibold tracking-widest uppercase"
        style={{ color: "var(--color-text-muted)", letterSpacing: "0.12em" }}
      >
        {tableLabel}
      </span>

      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1.5" style={{ color: "var(--color-text-muted)" }}>
          <Users className="size-3" aria-hidden />
          <span className="text-xs font-medium">{participants.length}</span>
        </div>
        <RealtimeIndicator />
      </div>
    </header>
  )
}
