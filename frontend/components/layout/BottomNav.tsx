"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { UtensilsCrossed, ClipboardList, Bell, Receipt } from "lucide-react"
import { useOrdersStore } from "@/store/orders"

interface BottomNavProps {
  sessionId: string
}

export function BottomNav({ sessionId }: BottomNavProps) {
  const pathname = usePathname()
  const hasOrders = useOrdersStore((s) => s.orders.length > 0)

  const tabs = [
    { href: `/session/${sessionId}/menu`,    label: "Menu",   icon: UtensilsCrossed, muted: false },
    { href: `/session/${sessionId}/orders`,  label: "Orders", icon: ClipboardList,   muted: false },
    { href: `/session/${sessionId}/payment`, label: "Bill",   icon: Receipt,         muted: !hasOrders },
    { href: `/session/${sessionId}/assist`,  label: "Help",   icon: Bell,            muted: false },
  ]

  return (
    <nav
      className="flex"
      style={{
        height: 84,
        background: "var(--bg-overlay)",
        backdropFilter: "blur(24px) saturate(140%)",
        WebkitBackdropFilter: "blur(24px) saturate(140%)",
        borderTop: "1px solid var(--line-2)",
      }}
      aria-label="Session navigation"
    >
      {tabs.map(({ href, label, icon: Icon, muted }) => {
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
              color: active ? "var(--accent)" : muted ? "var(--ink-4, var(--ink-3))" : "var(--ink-3)",
              fontWeight: active ? 600 : 400,
              opacity: muted && !active ? 0.5 : 1,
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
