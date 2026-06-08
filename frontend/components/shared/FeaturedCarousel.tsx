import { useState } from "react"
import { Vignette } from "@/components/shared/Vignette"
import { formatCurrency } from "@/lib/format"
import type { MenuItem } from "@/types/api"

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

function FeaturedCard({ item, onSelect }: { item: MenuItem; onSelect: (i: MenuItem) => void }) {
  const [imgError, setImgError] = useState(false)
  return (
    <button
      onClick={() => onSelect(item)}
      className="press"
      style={{
        width: 140,
        flexShrink: 0,
        borderRadius: 14,
        background: "var(--bg-elev-2)",
        border: "1px solid var(--line-2)",
        padding: 0,
        overflow: "hidden",
        textAlign: "left",
        boxShadow: "var(--shadow-2)",
      }}
    >
      <div style={{
        height: 3,
        background: "linear-gradient(90deg, var(--accent), var(--accent-soft) 70%, transparent)",
      }} />
      <div style={{ padding: "12px 12px 14px" }}>
        <div style={{ marginBottom: 12, display: "flex", justifyContent: "center" }}>
          {item.image_url && !imgError ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={item.image_url}
              alt=""
              aria-hidden
              loading="lazy"
              onError={() => setImgError(true)}
              style={{ width: 64, height: 64, borderRadius: 10, objectFit: "cover" }}
            />
          ) : (
            <Vignette hue={itemHue(item.id)} size={64} />
          )}
        </div>
        <div
          className="serif"
          style={{
            fontSize: 16,
            fontWeight: 500,
            color: "var(--ink-1)",
            lineHeight: 1.2,
            marginBottom: 4,
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {item.name}
        </div>
        <div className="serif" style={{ fontSize: 14, color: "var(--accent)", fontWeight: 500 }}>
          {formatCurrency(item.price)}
        </div>
      </div>
    </button>
  )
}

interface Props {
  items: MenuItem[]
  onSelect: (item: MenuItem) => void
}

export function FeaturedCarousel({ items, onSelect }: Props) {
  if (items.length === 0) return null

  return (
    <div style={{ paddingBottom: 8 }}>
      <div style={{ padding: "0 20px 10px" }}>
        <span className="eyebrow">From the kitchen</span>
      </div>
      <div className="hscroll" style={{ paddingLeft: 20, paddingRight: 20, gap: 12, display: "flex" }}>
        {items.map((item) => (
          <FeaturedCard key={item.id} item={item} onSelect={onSelect} />
        ))}
      </div>
    </div>
  )
}
