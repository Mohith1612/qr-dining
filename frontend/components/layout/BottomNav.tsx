"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { UtensilsCrossed, ClipboardList, Bell } from "lucide-react"

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
      className="flex"
      style={{
        height: 84,
        background: "color-mix(in srgb, var(--bg-base) 88%, transparent)",
        backdropFilter: "blur(24px) saturate(120%)",
        WebkitBackdropFilter: "blur(24px) saturate(120%)",
        borderTop: "1px solid var(--line-1)",
      }}
      aria-label="Session navigation"
    >
      {tabs.map(({ href, label, icon: Icon }) => {
        const active = pathname === href
        return (
          <Link
            key={href}
            href={href}
            className="press flex flex-1 flex-col items-center justify-start"
            style={{
              paddingTop: 10,
              paddingBottom: 24,
              gap: 5,
              color: active ? "var(--accent)" : "var(--ink-3)",
              fontWeight: active ? 600 : 400,
              WebkitTapHighlightColor: "transparent",
            }}
            aria-current={active ? "page" : undefined}
          >
            <Icon size={22} strokeWidth={active ? 1.8 : 1.4} aria-hidden />
            <span style={{ fontSize: 10.5, letterSpacing: "0.04em" }}>{label}</span>
          </Link>
        )
      })}
    </nav>
  )
}
