"use client"

import { ChevronRight } from "lucide-react"

interface FloatingCartProps {
  /** number of items currently in the cart */
  itemCount: number
  /** pre-formatted total, e.g. "₹1,810.00" */
  total: string
  onClick: () => void
  /** button label; defaults to "View Order" */
  label?: string
}

/**
 * Persistent glassmorphic cart pill that floats above the bottom nav.
 * Serene "Floating Cart": ~80% cream + heavy backdrop blur, charcoal action.
 * Reused on the menu (A5) and cart/review (A6) screens.
 */
export function FloatingCart({ itemCount, total, onClick, label = "View Order" }: FloatingCartProps) {
  if (itemCount <= 0) return null

  return (
    <div
      className="fixed left-0 right-0 z-30 flex justify-center px-4"
      style={{
        bottom: "calc(84px + env(safe-area-inset-bottom) + 12px)",
        pointerEvents: "none",
      }}
    >
      <button
        type="button"
        onClick={onClick}
        aria-label={`${label} — ${itemCount} ${itemCount === 1 ? "item" : "items"}, ${total}`}
        className="press"
        style={{
          pointerEvents: "auto",
          width: "min(100%, 440px)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          padding: "10px 10px 10px 18px",
          borderRadius: "var(--rad-pill)",
          background: "var(--bg-overlay)",
          backdropFilter: "blur(20px) saturate(140%)",
          WebkitBackdropFilter: "blur(20px) saturate(140%)",
          border: "1px solid var(--line-2)",
          boxShadow: "var(--shadow-3)",
          cursor: "pointer",
        }}
      >
        <span style={{ display: "flex", flexDirection: "column", alignItems: "flex-start", lineHeight: 1.15 }}>
          <span style={{ fontSize: 10.5, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--ink-3)" }}>
            {itemCount} {itemCount === 1 ? "item" : "items"}
          </span>
          <span style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)", fontVariantNumeric: "tabular-nums" }}>
            {total}
          </span>
        </span>
        <span
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 6,
            padding: "10px 16px",
            borderRadius: "var(--rad-pill)",
            background: "var(--accent)",
            color: "var(--accent-ink)",
            fontSize: 13.5,
            fontWeight: 600,
            letterSpacing: "0.01em",
            whiteSpace: "nowrap",
          }}
        >
          {label}
          <ChevronRight size={16} aria-hidden />
        </span>
      </button>
    </div>
  )
}
