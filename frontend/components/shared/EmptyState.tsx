import type { LucideIcon } from "lucide-react"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  eyebrow?: string
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({ icon: Icon, title, eyebrow, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn("atmos flex flex-col items-center justify-center py-20 px-8 text-center gap-4", className)}
      style={{ background: "var(--glow-warm)" }}
    >
      <div
        className="flex items-center justify-center mb-2"
        style={{
          width: 88,
          height: 88,
          borderRadius: "var(--rad-xl)",
          background: "linear-gradient(145deg, var(--bg-elev-2), var(--bg-elev-3))",
          boxShadow: "var(--shadow-3), inset 0 0 0 1px var(--accent-soft)",
          flexShrink: 0,
        }}
      >
        <Icon className="size-9" style={{ color: "var(--accent)" }} aria-hidden />
      </div>
      {eyebrow && <p className="eyebrow">{eyebrow}</p>}
      <h3
        className="serif leading-snug"
        style={{ fontSize: 28, fontWeight: 500, color: "var(--ink-1)" }}
      >
        {title}
      </h3>
      {description && (
        <p className="text-sm max-w-xs leading-relaxed" style={{ color: "var(--ink-2)" }}>
          {description}
        </p>
      )}
      {action && (
        <button
          onClick={action.onClick}
          className="press mt-2 text-sm font-medium px-5 py-2.5"
          style={{
            backgroundColor: "var(--accent)",
            color: "var(--accent-ink)",
            borderRadius: "var(--rad-md)",
          }}
        >
          {action.label}
        </button>
      )}
    </div>
  )
}
