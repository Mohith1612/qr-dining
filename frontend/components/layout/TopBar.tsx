"use client"

import { Users } from "lucide-react"
import { useSessionStore } from "@/store/session"
import { RealtimeIndicator } from "@/components/shared/RealtimeIndicator"

export function TopBar() {
  const session = useSessionStore((s) => s.session)
  const participants = useSessionStore((s) => s.participants)

  const tableLabel = session ? `Table ${session.table_id}` : "Table"

  return (
    <header
      className="flex items-center justify-between px-4 py-3 border-b sticky top-0 z-30"
      style={{
        backgroundColor: "var(--color-surface)",
        borderColor: "var(--color-border)",
        paddingTop: "calc(0.75rem + env(safe-area-inset-top))",
      }}
    >
      <div className="flex items-center gap-2">
        <span
          className="text-xs font-semibold uppercase tracking-wider px-2 py-0.5 rounded-full"
          style={{
            backgroundColor: "var(--color-bg)",
            color: "var(--color-text-muted)",
            border: "1px solid var(--color-border)",
          }}
        >
          {tableLabel}
        </span>
      </div>

      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1" style={{ color: "var(--color-text-muted)" }}>
          <Users className="size-3.5" aria-hidden />
          <span className="text-xs font-medium">{participants.length}</span>
        </div>
        <RealtimeIndicator />
      </div>
    </header>
  )
}
