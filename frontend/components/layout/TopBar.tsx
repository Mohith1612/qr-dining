"use client"

import { Utensils } from "lucide-react"
import { useSessionStore } from "@/store/session"

export function TopBar() {
  const session = useSessionStore((s) => s.session)
  const participants = useSessionStore((s) => s.participants)

  const tableLabel = session
    ? (session.table_identifier ?? `Table ${session.table_id}`)
    : "Table"

  return (
    <header
      className="flex items-center justify-between px-4 sticky top-0 z-30"
      style={{
        height: 56,
        background: "var(--bg-base)",
        borderBottom: "1px solid var(--line-1)",
        paddingTop: "env(safe-area-inset-top)",
      }}
    >
      {/* Table pill */}
      <div
        className="flex items-center gap-2 press"
        style={{
          padding: "6px 12px",
          borderRadius: "var(--rad-pill)",
          background: "var(--bg-elev-1)",
          border: "1px solid var(--line-2)",
          boxShadow: "var(--shadow-1)",
        }}
      >
        <Utensils size={12} style={{ color: "var(--accent)" }} aria-hidden />
        <span style={{ fontSize: 11, fontWeight: 600, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--ink-1)" }}>
          {tableLabel}
        </span>
      </div>

      {/* Right slot */}
      <div className="flex items-center gap-2.5" style={{ color: "var(--ink-3)" }}>
        <span style={{ fontSize: 12, fontWeight: 500 }}>{participants.length}</span>
        <span className="live-dot" aria-hidden />
      </div>
    </header>
  )
}
