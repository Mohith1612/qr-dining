import { cn } from "@/lib/utils"
import type { CSSProperties } from "react"

interface SectionHeaderProps {
  children: React.ReactNode
  className?: string
  style?: CSSProperties
}

export function SectionHeader({ children, className, style }: SectionHeaderProps) {
  return (
    <p className={cn("eyebrow", className)} style={style}>
      {children}
    </p>
  )
}
