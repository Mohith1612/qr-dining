"use client"

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts"
import type { OrderVolumeDay } from "@/lib/api/analytics"

type Props = {
  days: OrderVolumeDay[]
}

function formatDay(dateStr: string): string {
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString("en", { month: "short", day: "numeric" })
  } catch {
    return dateStr
  }
}

export function OrderVolumeChart({ days }: Props) {
  if (days.length === 0) {
    return (
      <p style={{ color: "var(--color-text-muted)", fontSize: "14px", textAlign: "center", padding: "24px 0" }}>
        No order data in this period.
      </p>
    )
  }

  const data = days.map(d => ({
    label: formatDay(d.day),
    orders: d.order_count,
    revenue: parseFloat(d.revenue),
  }))

  return (
    <ResponsiveContainer width="100%" height={160}>
      <LineChart data={data} margin={{ top: 4, right: 8, left: -24, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
        <XAxis
          dataKey="label"
          tick={{ fontSize: 10, fill: "var(--color-text-muted)" }}
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
          formatter={(value, name) => {
            const num = Number(value)
            return [
              String(name) === "revenue" ? `₹${num.toFixed(2)}` : num,
              String(name) === "revenue" ? "Revenue" : "Orders",
            ]
          }}
          cursor={{ stroke: "var(--color-accent)", strokeWidth: 1 }}
        />
        <Line
          type="monotone"
          dataKey="orders"
          stroke="var(--color-accent)"
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 4, fill: "var(--color-accent)" }}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
