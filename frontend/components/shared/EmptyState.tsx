import type { LucideIcon } from "lucide-react"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn("flex flex-col items-center justify-center py-16 px-6 text-center gap-3", className)}
    >
      <div
        className="size-12 rounded-2xl flex items-center justify-center mb-1"
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
        }}
      >
        <Icon className="size-5" style={{ color: "var(--color-text-muted)" }} aria-hidden />
      </div>
      <h3
        className="text-xl font-medium leading-snug"
        style={{
          fontFamily: "var(--font-display)",
          color: "var(--color-text)",
        }}
      >
        {title}
      </h3>
      {description && (
        <p className="text-sm max-w-xs" style={{ color: "var(--color-text-muted)" }}>
          {description}
        </p>
      )}
      {action && (
        <button
          onClick={action.onClick}
          className="mt-2 text-sm font-medium px-4 py-2 rounded-xl transition-opacity active:opacity-70"
          style={{
            backgroundColor: "var(--color-accent)",
            color: "var(--color-accent-fg)",
          }}
        >
          {action.label}
        </button>
      )}
    </div>
  )
}
