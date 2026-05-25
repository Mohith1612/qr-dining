"use client"

import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from "recharts"
import type { BusyHour } from "@/lib/api/analytics"

type Props = {
  hours: BusyHour[]
}

function fillHours(hours: BusyHour[]): { hour: number; label: string; order_count: number }[] {
  const map = new Map(hours.map(h => [h.hour, h.order_count]))
  return Array.from({ length: 24 }, (_, i) => ({
    hour: i,
    label: i === 0 ? "12a" : i === 12 ? "12p" : i < 12 ? `${i}a` : `${i - 12}p`,
    order_count: map.get(i) ?? 0,
  }))
}

export function BusyHoursChart({ hours }: Props) {
  const data = fillHours(hours)
  const max = Math.max(...data.map(d => d.order_count), 1)

  if (hours.length === 0) {
    return (
      <p style={{ color: "var(--ink-3)", fontSize: 13, textAlign: "center", padding: "24px 0" }}>
        No order data in this period.
      </p>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={160}>
      <BarChart data={data} margin={{ top: 0, right: 0, left: -28, bottom: 0 }}>
        <XAxis
          dataKey="label"
          tick={{ fontSize: 9, fill: "var(--ink-4)" }}
          interval={3}
          axisLine={false}
          tickLine={false}
        />
        <YAxis
          tick={{ fontSize: 9, fill: "var(--ink-4)" }}
          axisLine={false}
          tickLine={false}
          allowDecimals={false}
        />
        <Tooltip
          contentStyle={{
            background: "var(--bg-elev-3)",
            border: "1px solid var(--line-2)",
            borderRadius: "var(--rad-md)",
            fontSize: 12,
            color: "var(--ink-1)",
            boxShadow: "var(--shadow-2)",
          }}
          formatter={(value) => [Number(value), "Orders"]}
          labelFormatter={(label: string) => label}
          cursor={{ fill: "var(--line-1)" }}
        />
        <Bar dataKey="order_count" radius={[3, 3, 0, 0]}>
          {data.map((entry, i) => (
            <Cell
              key={i}
              fill={entry.order_count === max && max > 0 ? "var(--accent)" : "var(--line-2)"}
              opacity={entry.order_count > 0 ? 1 : 0.5}
            />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}
