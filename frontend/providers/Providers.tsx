"use client"

import { useEffect } from "react"
import { usePathname } from "next/navigation"
import { TenantProvider, useTenant } from "@/providers/TenantProvider"
import { ThemeProvider } from "@/providers/ThemeProvider"
import { Toaster } from "@/components/ui/sonner"
import { applyTheme, validPreset, type ThemeConfig } from "@/lib/theme/applyTheme"

// Applies the tenant's structured theme (preset + custom tokens) once resolved.
// Server-driven branding is authoritative and applied directly to <html>, so it
// never leaks across tenants via localStorage. Fallback chain: structured theme →
// legacy settings.theme preset → frontend default (next-themes).
function TenantThemeSync() {
  const { ready, theme, settings } = useTenant()
  const pathname = usePathname()

  useEffect(() => {
    if (!ready) return
    // Guest routes only. Staff/platform are not tenant-branded: they set a static
    // data-surface and must never have a tenant theme applied to <html> (this is
    // what made the staff theme reset to dark on refresh).
    if (pathname?.startsWith("/staff") || pathname?.startsWith("/platform")) return
    let config: ThemeConfig | null = theme
    if (!config) {
      const legacy = typeof settings?.theme === "string" ? settings.theme : null
      if (legacy && validPreset(legacy)) config = { preset: legacy, tokens: {} }
    }
    if (config) applyTheme(config)
  }, [ready, theme, settings, pathname])

  return null
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <TenantProvider>
      <ThemeProvider>
        <TenantThemeSync />
        {children}
        <Toaster position="top-center" />
      </ThemeProvider>
    </TenantProvider>
  )
}
