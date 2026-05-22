"use client"

import { useEffect } from "react"
import { useTheme } from "next-themes"
import { TenantProvider, useTenant } from "@/providers/TenantProvider"
import { ThemeProvider } from "@/providers/ThemeProvider"
import { Toaster } from "@/components/ui/sonner"

// Syncs the tenant's preferred theme on first load, only if the user has not
// already set a theme preference in localStorage.
function TenantThemeSync() {
  const { ready, settings } = useTenant()
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    if (!ready) return
    const tenantTheme = typeof settings?.theme === "string" ? settings.theme : null
    if (!tenantTheme) return

    // Apply tenant theme only if the current theme is still the default
    // (i.e. no user override in localStorage).
    const stored = localStorage.getItem("qr-dining-theme")
    if (!stored && theme !== tenantTheme) {
      setTheme(tenantTheme)
    }
  }, [ready, settings, theme, setTheme])

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
