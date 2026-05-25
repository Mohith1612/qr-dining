"use client"

import type { TopItem } from "@/lib/api/analytics"

type Props = {
  items: TopItem[]
}

export function TopItemsList({ items }: Props) {
  if (items.length === 0) {
    return (
      <p style={{ color: "var(--ink-3)", fontSize: 13, textAlign: "center", padding: "24px 0" }}>
        No orders in this period.
      </p>
    )
  }

  const max = Math.max(...items.map(i => i.total_quantity), 1)

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      {items.map((item, idx) => (
        <div key={item.menu_item_id} style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <span
            className={idx === 0 ? "serif" : ""}
            style={{
              width: 22,
              fontSize: idx === 0 ? 15 : 12,
              fontWeight: 600,
              color: idx === 0 ? "var(--accent)" : "var(--ink-4)",
              flexShrink: 0,
              textAlign: "right",
            }}
          >
            {idx + 1}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 5 }}>
              <span
                style={{
                  fontSize: 14,
                  fontWeight: 500,
                  color: "var(--ink-1)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {item.menu_item_name}
              </span>
              <span style={{ fontSize: 12, color: "var(--ink-3)", flexShrink: 0, marginLeft: 8 }}>
                {item.total_quantity}×
              </span>
            </div>
            <div
              style={{
                height: 3,
                borderRadius: 2,
                background: "var(--line-2)",
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  height: "100%",
                  width: `${(item.total_quantity / max) * 100}%`,
                  background: idx === 0 ? "var(--accent)" : "var(--ink-4)",
                  borderRadius: 2,
                  transition: `width 0.4s var(--ease-out)`,
                }}
              />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
