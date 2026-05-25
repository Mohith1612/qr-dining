import { cn } from "@/lib/utils"

interface SectionHeaderProps {
  children: React.ReactNode
  className?: string
}

export function SectionHeader({ children, className }: SectionHeaderProps) {
  return (
    <p className={cn("eyebrow", className)}>
      {children}
    </p>
  )
}
