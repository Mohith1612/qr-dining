"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import {
  LayoutDashboard,
  Building2,
  Package,
  KeyRound,
  BarChart3,
  Flag,
  Palette,
  LifeBuoy,
  type LucideIcon,
} from "lucide-react"

type NavItem = { href: string; label: string; icon: LucideIcon }

const NAV: NavItem[] = [
  { href: "/platform", label: "Overview", icon: LayoutDashboard },
  { href: "/platform/support", label: "Support", icon: LifeBuoy },
  { href: "/platform/organizations", label: "Organizations", icon: Building2 },
  { href: "/platform/plans", label: "Plans", icon: Package },
  { href: "/platform/entitlements", label: "Entitlements", icon: KeyRound },
  { href: "/platform/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/platform/feature-flags", label: "Feature Flags", icon: Flag },
  { href: "/platform/themes", label: "Themes", icon: Palette },
]

function isActive(pathname: string, href: string): boolean {
  if (href === "/platform") return pathname === "/platform"
  return pathname === href || pathname.startsWith(href + "/")
}

export function PlatformNav() {
  const pathname = usePathname()

  return (
    <nav style={{ display: "flex", flexDirection: "column", gap: 2, padding: "8px 12px" }}>
      {NAV.map((item) => {
        const active = isActive(pathname, item.href)
        const Icon = item.icon
        return (
          <Link
            key={item.href}
            href={item.href}
            className="press"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 10,
              padding: "9px 12px",
              borderRadius: "var(--rad-md)",
              fontSize: 14,
              fontWeight: active ? 600 : 400,
              textDecoration: "none",
              background: active ? "var(--bg-elev-2)" : "transparent",
              color: active ? "var(--ink-1)" : "var(--ink-3)",
              border: active ? "1px solid var(--line-2)" : "1px solid transparent",
              transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease)",
            }}
          >
            <Icon size={17} style={{ color: active ? "var(--accent)" : "var(--ink-4)" }} aria-hidden />
            {item.label}
          </Link>
        )
      })}
    </nav>
  )
}
