export type TagVariant = "veg" | "signature" | "chef" | "slow" | "seasonal" | "spice-1" | "spice-2"

interface TagProps {
  variant: TagVariant
  label?: string
}

const CONFIG: Record<TagVariant, { bg: string; fg: string; label: string }> = {
  veg:        { bg: "var(--ok-soft)",     fg: "var(--ok)",     label: "Vegetarian"  },
  signature:  { bg: "var(--accent-soft)", fg: "var(--accent)", label: "Signature"   },
  chef:       { bg: "var(--accent-soft)", fg: "var(--accent)", label: "Chef's pick" },
  slow:       { bg: "var(--info-soft)",   fg: "var(--info)",   label: "Slow-cooked" },
  seasonal:   { bg: "var(--accent-soft)", fg: "var(--accent)", label: "Seasonal"    },
  "spice-1":  { bg: "var(--warn-soft)",   fg: "var(--warn)",   label: "Mild heat"   },
  "spice-2":  { bg: "var(--alert-soft)",  fg: "var(--alert)",  label: "Spiced"      },
}

export function Tag({ variant, label }: TagProps) {
  const cfg = CONFIG[variant]
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        padding: "3px 8px 3px 7px",
        borderRadius: "var(--rad-pill)",
        background: cfg.bg,
        color: cfg.fg,
        fontSize: 10.5,
        fontWeight: 500,
        letterSpacing: "0.02em",
        lineHeight: 1.2,
        whiteSpace: "nowrap",
      }}
    >
      <span style={{ width: 5, height: 5, borderRadius: "50%", backgroundColor: cfg.fg, flexShrink: 0, display: "inline-block" }} />
      {label ?? cfg.label}
    </span>
  )
}
