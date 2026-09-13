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
  Rocket,
  Gauge,
  QrCode,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react"
import { usePlatformStore } from "@/store/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { PlatformRole } from "@/types/platform"

// roles omitted = visible to every platform role. When present, the item shows
// only if the operator holds one of them (super_admin always passes). This keeps
// non-super operators from clicking into surfaces that reject them or render an
// empty read-only page — the "felt buggy" symptom from manual testing.
type NavItem = { href: string; label: string; icon: LucideIcon; roles?: PlatformRole[] }

const NAV: NavItem[] = [
  { href: "/platform", label: "Overview", icon: LayoutDashboard },
  { href: "/platform/onboarding", label: "Onboard", icon: Rocket, roles: [] },
  { href: "/platform/support", label: "Support", icon: LifeBuoy, roles: ["support_admin", "read_only_auditor"] },
  { href: "/platform/organizations", label: "Organizations", icon: Building2 },
  { href: "/platform/plans", label: "Plans", icon: Package, roles: ["billing_admin"] },
  { href: "/platform/entitlements", label: "Entitlements", icon: KeyRound, roles: [] },
  { href: "/platform/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/platform/observability", label: "Observability", icon: Gauge },
  { href: "/platform/feature-flags", label: "Feature Flags", icon: Flag, roles: [] },
  { href: "/platform/themes", label: "Themes", icon: Palette, roles: [] },
  { href: "/platform/collateral", label: "Collateral", icon: QrCode, roles: [] },
  { href: "/platform/security", label: "Security", icon: ShieldCheck },
]

function isActive(pathname: string, href: string): boolean {
  if (href === "/platform") return pathname === "/platform"
  return pathname === href || pathname.startsWith(href + "/")
}

export function PlatformNav() {
  const pathname = usePathname()
  const roles = usePlatformStore((s) => s.roles)
  const visible = NAV.filter((item) => item.roles === undefined || hasPlatformRole(roles, ...item.roles))

  return (
    <nav style={{ display: "flex", flexDirection: "column", gap: 2, padding: "8px 12px" }}>
      {visible.map((item) => {
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
