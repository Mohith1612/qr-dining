import { cn } from "@/lib/utils"
import type { CSSProperties, HTMLAttributes } from "react"

type CardVariant = "default" | "elevated" | "interactive" | "inset"

interface HospitalityCardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: CardVariant
  style?: CSSProperties
}

const surfaceByVariant: Record<CardVariant, string> = {
  default: "var(--color-surface)",
  elevated: "var(--color-surface-raised)",
  interactive: "var(--color-surface)",
  inset: "var(--color-surface-inset)",
}

const shadowByVariant: Record<CardVariant, string> = {
  default: "var(--shadow-card)",
  elevated: "var(--shadow-elevated)",
  interactive: "var(--shadow-card)",
  inset: "none",
}

export function HospitalityCard({
  variant = "default",
  className,
  style,
  children,
  ...props
}: HospitalityCardProps) {
  return (
    <div
      className={cn(
        "rounded-2xl border",
        variant === "interactive" && "transition-opacity active:opacity-70 cursor-pointer",
        className
      )}
      style={{
        backgroundColor: surfaceByVariant[variant],
        borderColor: "var(--color-border)",
        borderRadius: "var(--radius-lg)",
        boxShadow: shadowByVariant[variant],
        padding: "var(--space-card)",
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  )
}
