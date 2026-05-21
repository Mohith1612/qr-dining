"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { UtensilsCrossed, ClipboardList, Bell } from "lucide-react"
import { cn } from "@/lib/utils"

interface BottomNavProps {
  sessionId: string
}

export function BottomNav({ sessionId }: BottomNavProps) {
  const pathname = usePathname()

  const tabs = [
    { href: `/session/${sessionId}/menu`, label: "Menu", icon: UtensilsCrossed },
    { href: `/session/${sessionId}/orders`, label: "Orders", icon: ClipboardList },
    { href: `/session/${sessionId}/assist`, label: "Help", icon: Bell },
  ]

  return (
    <nav
      className="flex border-t"
      style={{
        backgroundColor: "var(--color-surface)",
        borderColor: "var(--color-border)",
      }}
      aria-label="Session navigation"
    >
      {tabs.map(({ href, label, icon: Icon }) => {
        const active = pathname === href
        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "flex flex-1 flex-col items-center justify-center gap-1 py-2 text-xs font-medium transition-colors min-h-[3rem]",
              active ? "opacity-100" : "opacity-50 hover:opacity-75"
            )}
            style={{ color: active ? "var(--color-accent)" : "var(--color-text-muted)" }}
            aria-current={active ? "page" : undefined}
          >
            <Icon className="size-5" aria-hidden />
            <span>{label}</span>
          </Link>
        )
      })}
    </nav>
  )
}
