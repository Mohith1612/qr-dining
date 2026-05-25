"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { UtensilsCrossed, ClipboardList, Bell } from "lucide-react"
import { motion } from "framer-motion"
import { springSnappy } from "@/lib/motion"
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
          <motion.div
            key={href}
            className="flex flex-1"
            whileTap={{ scale: 0.92 }}
            transition={springSnappy}
          >
            <Link
              href={href}
              className={cn(
                "flex flex-1 flex-col items-center justify-center gap-1 py-2.5 text-xs font-medium min-h-[3.25rem]",
                active ? "" : "opacity-45 hover:opacity-65 transition-opacity"
              )}
              style={{ color: active ? "var(--color-accent)" : "var(--color-text-muted)" }}
              aria-current={active ? "page" : undefined}
            >
              <Icon className="size-[1.125rem]" aria-hidden />
              <span style={{ letterSpacing: "0.03em", fontSize: "10px" }}>{label}</span>
            </Link>
          </motion.div>
        )
      })}
    </nav>
  )
}
