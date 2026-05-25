"use client"

import type { AnalyticsPeriod } from "@/lib/api/analytics"

const PERIODS: { label: string; value: AnalyticsPeriod }[] = [
  { label: "Today",  value: "daily"   },
  { label: "7 Days", value: "weekly"  },
  { label: "30 Days",value: "monthly" },
]

type Props = {
  value: AnalyticsPeriod
  onChange: (period: AnalyticsPeriod) => void
}

export function PeriodSelector({ value, onChange }: Props) {
  return (
    <div
      style={{
        display: "flex",
        gap: 3,
        padding: 3,
        background: "var(--bg-elev-1)",
        borderRadius: "var(--rad-pill)",
        border: "1px solid var(--line-1)",
        width: "fit-content",
      }}
    >
      {PERIODS.map(p => {
        const active = value === p.value
        return (
          <button
            key={p.value}
            onClick={() => onChange(p.value)}
            className="press"
            style={{
              padding: "7px 16px",
              borderRadius: "var(--rad-pill)",
              border: "none",
              cursor: "pointer",
              fontSize: 13,
              fontWeight: active ? 600 : 400,
              background: active ? "var(--bg-elev-3)" : "transparent",
              color: active ? "var(--ink-1)" : "var(--ink-3)",
              boxShadow: active ? "var(--shadow-1)" : "none",
              transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), box-shadow var(--dur-fast) var(--ease)",
              whiteSpace: "nowrap",
            }}
          >
            {p.label}
          </button>
        )
      })}
    </div>
  )
}
