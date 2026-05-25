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
      <p style={{ color: "var(--ink-3)", fontSize: 13, textAlign: "center", padding: "24px 0" }}>
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
      <LineChart data={data} margin={{ top: 4, right: 8, left: -28, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--line-1)" vertical={false} />
        <XAxis
          dataKey="label"
          tick={{ fontSize: 9, fill: "var(--ink-4)" }}
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
          formatter={(value, name) => {
            const num = Number(value)
            return [
              String(name) === "revenue" ? `₹${num.toFixed(2)}` : num,
              String(name) === "revenue" ? "Revenue" : "Orders",
            ]
          }}
          cursor={{ stroke: "var(--accent)", strokeWidth: 1, strokeDasharray: "4 2" }}
        />
        <Line
          type="monotone"
          dataKey="orders"
          stroke="var(--accent)"
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 4, fill: "var(--accent)", strokeWidth: 0 }}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
