import type { CSSProperties } from "react"
import type { OrderStatus, AssistanceStatus } from "@/types/api"

type Status = OrderStatus | AssistanceStatus

interface StatusStyle {
  label: string
  style: CSSProperties
}

function makeStyle(bg: string, color: string): CSSProperties {
  return {
    backgroundColor: bg,
    color,
    border: `1px solid ${bg}`,
  }
}

const STATUS_MAP: Record<Status, StatusStyle> = {
  pending: {
    label: "Pending",
    style: makeStyle("color-mix(in oklch, var(--color-warning) 18%, transparent)", "var(--color-warning)"),
  },
  confirmed: {
    label: "Confirmed",
    style: makeStyle("color-mix(in oklch, var(--color-accent) 18%, transparent)", "var(--color-accent)"),
  },
  preparing: {
    label: "Preparing",
    style: makeStyle("color-mix(in oklch, var(--color-accent) 22%, transparent)", "var(--color-accent)"),
  },
  ready: {
    label: "Ready",
    style: makeStyle("color-mix(in oklch, var(--color-success) 20%, transparent)", "var(--color-success)"),
  },
  served: {
    label: "Served",
    style: makeStyle("color-mix(in oklch, var(--color-text-muted) 12%, transparent)", "var(--color-text-muted)"),
  },
  cancelled: {
    label: "Cancelled",
    style: makeStyle("color-mix(in oklch, var(--color-error) 16%, transparent)", "var(--color-error)"),
  },
  acknowledged: {
    label: "On the way",
    style: makeStyle("color-mix(in oklch, var(--color-accent) 18%, transparent)", "var(--color-accent)"),
  },
  resolved: {
    label: "Resolved",
    style: makeStyle("color-mix(in oklch, var(--color-text-muted) 12%, transparent)", "var(--color-text-muted)"),
  },
}

interface StatusBadgeProps {
  status: Status
  className?: string
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const entry = STATUS_MAP[status] ?? {
    label: status,
    style: makeStyle("color-mix(in oklch, var(--color-text-muted) 12%, transparent)", "var(--color-text-muted)"),
  }

  return (
    <span
      className={className}
      style={{
        display: "inline-flex",
        alignItems: "center",
        fontSize: "11px",
        fontWeight: 500,
        letterSpacing: "0.02em",
        padding: "2px 8px",
        borderRadius: "100px",
        ...entry.style,
      }}
    >
      {entry.label}
    </span>
  )
}
