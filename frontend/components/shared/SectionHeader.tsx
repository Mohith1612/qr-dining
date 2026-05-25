import { cn } from "@/lib/utils"

interface SectionHeaderProps {
  children: React.ReactNode
  className?: string
}

export function SectionHeader({ children, className }: SectionHeaderProps) {
  return (
    <p
      className={cn("text-xs font-semibold uppercase tracking-wider", className)}
      style={{ color: "var(--color-text-muted)" }}
    >
      {children}
    </p>
  )
}
