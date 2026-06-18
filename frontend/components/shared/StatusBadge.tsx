import type { OrderStatus, AssistanceStatus } from "@/types/api"

type Status = OrderStatus | AssistanceStatus

interface StatusConfig {
  label: string
  bg: string
  fg: string
}

const STATUS_MAP: Record<Status, StatusConfig> = {
  pending:      { label: "Pending",    bg: "var(--warn-soft)",   fg: "var(--warn)"   },
  confirmed:    { label: "Confirmed",  bg: "var(--info-soft)",   fg: "var(--info)"   },
  preparing:    { label: "Preparing",  bg: "var(--accent-soft)", fg: "var(--accent)" },
  ready:        { label: "Ready",      bg: "var(--ok-soft)",     fg: "var(--ok)"     },
  served:       { label: "Served",     bg: "var(--line-2)",      fg: "var(--ink-3)"  },
  cancelled:    { label: "Cancelled",  bg: "var(--alert-soft)",  fg: "var(--alert)"  },
  acknowledged: { label: "On the way", bg: "var(--accent-soft)", fg: "var(--accent)" },
  resolved:     { label: "Resolved",   bg: "var(--line-2)",      fg: "var(--ink-3)"  },
}

interface StatusBadgeProps {
  status: Status
  className?: string
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const cfg = STATUS_MAP[status] ?? { label: status, bg: "var(--line-2)", fg: "var(--ink-3)" }

  return (
    <span
      className={className}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        fontSize: 11,
        fontWeight: 500,
        textTransform: "uppercase",
        letterSpacing: "0.03em",
        padding: "3px 8px 3px 7px",
        borderRadius: "var(--rad-pill)",
        backgroundColor: cfg.bg,
        color: cfg.fg,
        lineHeight: 1.3,
        whiteSpace: "nowrap",
      }}
    >
      <span style={{ width: 5, height: 5, borderRadius: "50%", backgroundColor: cfg.fg, flexShrink: 0, display: "inline-block" }} />
      {cfg.label}
    </span>
  )
}
