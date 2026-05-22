"use client"

import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from "recharts"
import type { BusyHour } from "@/lib/api/analytics"

type Props = {
  hours: BusyHour[]
}

// Fill sparse hourly data with 0s for all 24 hours.
function fillHours(hours: BusyHour[]): { hour: number; label: string; order_count: number }[] {
  const map = new Map(hours.map(h => [h.hour, h.order_count]))
  return Array.from({ length: 24 }, (_, i) => ({
    hour: i,
    label: i === 0 ? "12am" : i === 12 ? "12pm" : i < 12 ? `${i}am` : `${i - 12}pm`,
    order_count: map.get(i) ?? 0,
  }))
}

export function BusyHoursChart({ hours }: Props) {
  const data = fillHours(hours)
  const max = Math.max(...data.map(d => d.order_count), 1)

  if (hours.length === 0) {
    return (
      <p style={{ color: "var(--color-text-muted)", fontSize: "14px", textAlign: "center", padding: "24px 0" }}>
        No order data in this period.
      </p>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={160}>
      <BarChart data={data} margin={{ top: 0, right: 0, left: -24, bottom: 0 }}>
        <XAxis
          dataKey="label"
          tick={{ fontSize: 10, fill: "var(--color-text-muted)" }}
          interval={3}
          axisLine={false}
          tickLine={false}
        />
        <YAxis
          tick={{ fontSize: 10, fill: "var(--color-text-muted)" }}
          axisLine={false}
          tickLine={false}
          allowDecimals={false}
        />
        <Tooltip
          contentStyle={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: "8px",
            fontSize: "12px",
            color: "var(--color-text)",
          }}
          formatter={(value) => [Number(value), "Orders"]}
          labelFormatter={(label: string) => label}
          cursor={{ fill: "var(--color-border)" }}
        />
        <Bar dataKey="order_count" radius={[3, 3, 0, 0]}>
          {data.map((entry, i) => (
            <Cell
              key={i}
              fill={entry.order_count === max && max > 0 ? "var(--color-accent)" : "var(--color-border)"}
              opacity={entry.order_count > 0 ? 1 : 0.4}
            />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}
