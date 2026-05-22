"use client"

import type { AnalyticsPeriod } from "@/lib/api/analytics"

const PERIODS: { label: string; value: AnalyticsPeriod }[] = [
  { label: "Today", value: "daily" },
  { label: "7 Days", value: "weekly" },
  { label: "30 Days", value: "monthly" },
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
        gap: "6px",
        padding: "4px",
        background: "var(--color-surface)",
        borderRadius: "var(--radius-lg)",
        border: "1px solid var(--color-border)",
        width: "fit-content",
      }}
    >
      {PERIODS.map(p => (
        <button
          key={p.value}
          onClick={() => onChange(p.value)}
          style={{
            padding: "6px 14px",
            borderRadius: "calc(var(--radius-lg) - 2px)",
            border: "none",
            cursor: "pointer",
            fontSize: "13px",
            fontWeight: value === p.value ? 600 : 400,
            background: value === p.value ? "var(--color-accent)" : "transparent",
            color: value === p.value ? "var(--color-accent-fg)" : "var(--color-text-muted)",
            transition: "all 0.15s ease",
          }}
        >
          {p.label}
        </button>
      ))}
    </div>
  )
}
