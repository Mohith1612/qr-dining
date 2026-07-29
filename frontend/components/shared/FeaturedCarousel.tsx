import { useState } from "react"
import { Vignette } from "@/components/shared/Vignette"
import { formatCurrency } from "@/lib/format"
import type { MenuItem } from "@/types/api"

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

function FeaturedCard({ item, onSelect, eyebrow }: { item: MenuItem; onSelect: (i: MenuItem) => void; eyebrow: string }) {
  const [imgError, setImgError] = useState(false)
  return (
    <button
      onClick={() => onSelect(item)}
      className="press"
      style={{
        width: "min(72vw, 280px)",
        flexShrink: 0,
        scrollSnapAlign: "start",
        borderRadius: "var(--rad-lg)",
        background: "var(--bg-elev-1)",
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
          {eyebrow}
        </div>
        <div
          className="serif"
          style={{
            fontSize: 18,
            fontWeight: 600,
            letterSpacing: "-0.01em",
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
  /** Header above the strip; pass null when the caller renders its own heading. */
  heading?: string | null
  /** Small label inside each card, e.g. "Chef's Selection". */
  cardEyebrow?: string
}

export function FeaturedCarousel({ items, onSelect, heading = "Featured", cardEyebrow = "Chef's Selection" }: Props) {
  if (items.length === 0) return null

  return (
    <section aria-label={heading ?? cardEyebrow} style={{ paddingBottom: 8 }}>
      {heading !== null && (
        <div
          style={{
            padding: "2px 20px 10px",
            display: "flex",
            alignItems: "baseline",
            justifyContent: "space-between",
          }}
        >
          <span className="eyebrow" style={{ color: "var(--ink-3)" }}>{heading}</span>
          {items.length > 3 && (
            <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{items.length} dishes</span>
          )}
        </div>
      )}
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
          <FeaturedCard key={item.id} item={item} onSelect={onSelect} eyebrow={cardEyebrow} />
        ))}
      </div>
    </section>
  )
}
