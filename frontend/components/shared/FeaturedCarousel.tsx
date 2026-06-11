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
        width: "min(72vw, 280px)",
        flexShrink: 0,
        scrollSnapAlign: "start",
        borderRadius: 14,
        background: "var(--bg-elev-2)",
        border: "1px solid var(--line-2)",
        padding: 0,
        overflow: "hidden",
        textAlign: "left",
        boxShadow: "var(--shadow-2)",
      }}
    >
      {item.image_url && !imgError ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={item.image_url}
          alt=""
          aria-hidden
          loading="lazy"
          onError={() => setImgError(true)}
          style={{
            width: "100%",
            height: 140,
            objectFit: "cover",
            objectPosition: "center top",
            display: "block",
          }}
        />
      ) : (
        <div
          style={{
            height: 140,
            background: "var(--bg-elev-3)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Vignette hue={itemHue(item.id)} size={72} />
        </div>
      )}
      <div style={{ padding: "14px 14px 16px" }}>
        <div
          className="eyebrow"
          style={{ marginBottom: 6, color: "var(--ink-4)" }}
        >
          Chef&apos;s Selection
        </div>
        <div
          className="serif"
          style={{
            fontSize: 18,
            fontWeight: 500,
            color: "var(--ink-1)",
            lineHeight: 1.25,
            marginBottom: 6,
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {item.name}
        </div>
        <div
          className="serif"
          style={{ fontSize: 15, color: "var(--accent)", fontWeight: 500, fontVariantNumeric: "tabular-nums" }}
        >
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
    <section aria-label="Featured dishes" style={{ paddingBottom: 8 }}>
      <div
        style={{
          padding: "2px 20px 10px",
          display: "flex",
          alignItems: "baseline",
          justifyContent: "space-between",
        }}
      >
        <span className="eyebrow" style={{ color: "var(--ink-3)" }}>Featured</span>
        {items.length > 3 && (
          <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{items.length} dishes</span>
        )}
      </div>
      <div
        className="hscroll presence"
        style={{
          display: "flex",
          gap: 16,
          paddingLeft: 20,
          paddingRight: 20,
          scrollSnapType: "x mandatory",
          scrollPaddingLeft: 20,
        }}
      >
        {items.map((item) => (
          <FeaturedCard key={item.id} item={item} onSelect={onSelect} />
        ))}
      </div>
    </section>
  )
}
