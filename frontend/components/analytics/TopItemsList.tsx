"use client"

import type { TopItem } from "@/lib/api/analytics"

type Props = {
  items: TopItem[]
}

export function TopItemsList({ items }: Props) {
  if (items.length === 0) {
    return (
      <p style={{ color: "var(--color-text-muted)", fontSize: "14px", textAlign: "center", padding: "24px 0" }}>
        No orders in this period.
      </p>
    )
  }

  const max = Math.max(...items.map(i => i.total_quantity), 1)

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
      {items.map((item, idx) => (
        <div key={item.menu_item_id} style={{ display: "flex", alignItems: "center", gap: "12px" }}>
          <span
            style={{
              width: "22px",
              fontSize: "12px",
              fontWeight: 600,
              color: idx === 0 ? "var(--color-accent)" : "var(--color-text-muted)",
              flexShrink: 0,
              textAlign: "right",
            }}
          >
            {idx + 1}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "4px" }}>
              <span
                style={{
                  fontSize: "14px",
                  fontWeight: 500,
                  color: "var(--color-text)",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {item.menu_item_name}
              </span>
              <span style={{ fontSize: "13px", color: "var(--color-text-muted)", flexShrink: 0, marginLeft: "8px" }}>
                {item.total_quantity}×
              </span>
            </div>
            <div
              style={{
                height: "4px",
                borderRadius: "2px",
                background: "var(--color-border)",
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  height: "100%",
                  width: `${(item.total_quantity / max) * 100}%`,
                  background: "var(--color-accent)",
                  borderRadius: "2px",
                  transition: "width 0.4s ease",
                }}
              />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
