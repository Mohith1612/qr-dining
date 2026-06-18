import { cn } from "@/lib/utils"
import type { CSSProperties, HTMLAttributes } from "react"

export type CardElev = 1 | 2 | 3

interface HospitalityCardProps extends HTMLAttributes<HTMLDivElement> {
  elev?: CardElev
  press?: boolean
  style?: CSSProperties
}

const bgByElev: Record<CardElev, string> = {
  1: "var(--bg-elev-1)",
  2: "var(--bg-elev-2)",
  3: "var(--bg-elev-3)",
}
const shadowByElev: Record<CardElev, string> = {
  1: "var(--shadow-1)",
  2: "var(--shadow-2)",
  3: "var(--shadow-3)",
}
const borderByElev: Record<CardElev, string> = {
  1: "1px solid var(--line-1)",
  2: "1px solid var(--line-2)",
  3: "1px solid var(--line-2)",
}

export function HospitalityCard({
  elev,
  press,
  className,
  style,
  children,
  ...props
}: HospitalityCardProps) {
  const level: CardElev = elev ?? 1

  return (
    <div
      className={cn(press && "press press-hover cursor-pointer", className)}
      style={{
        backgroundColor: bgByElev[level],
        boxShadow: shadowByElev[level],
        border: borderByElev[level],
        borderRadius: "var(--rad-lg)",
        transition: "transform var(--dur-fast) var(--ease), box-shadow var(--dur-fast) var(--ease)",
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  )
}
