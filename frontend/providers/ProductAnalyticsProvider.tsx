"use client"

import { useEffect, useRef, type ReactNode } from "react"
import { usePathname } from "next/navigation"
import { useTenant } from "@/providers/TenantProvider"
import { useStaffStore } from "@/store/staff"
import { usePlatformStore } from "@/store/platform"
import { useWsStore } from "@/store/ws"
import { ensureInit } from "@/lib/product-analytics/posthog"
import { appSurface, routeTemplate, sanitizeUrl } from "@/lib/product-analytics/routes"
import {
  identifyPlatform,
  identifyStaff,
  registerTenantProps,
  resetIdentity,
} from "@/lib/product-analytics/identity"
import { track } from "@/lib/product-analytics/events"

type Persona = "staff" | "platform" | null
let activePersona: Persona = null

function PageviewTracker({
  tenantReady,
  organizationId,
  restaurantSlug,
}: {
  tenantReady: boolean
  organizationId: number | null
  restaurantSlug: string | null
}) {
  const pathname = usePathname()
  const tenantRef = useRef({ tenantReady, organizationId, restaurantSlug })
  tenantRef.current = { tenantReady, organizationId, restaurantSlug }

  useEffect(() => {
    const client = ensureInit()
    if (!client || !pathname) return
    const surface = appSurface(pathname)
    const route = routeTemplate(pathname)
    const crossingTrustDomain =
      (surface === "guest" && activePersona !== null) ||
      (surface === "staff" && activePersona === "platform") ||
      (surface === "platform" && activePersona === "staff")
    if (crossingTrustDomain) {
      resetIdentity()
      activePersona = null
      const tenant = tenantRef.current
      if (surface === "guest" && tenant.tenantReady) {
        registerTenantProps({ orgId: tenant.organizationId, restaurantSlug: tenant.restaurantSlug })
      }
    }
    client.register({ app_surface: surface })
    track("$pageview", {
      $current_url: sanitizeUrl(window.location.href),
      $pathname: route,
      route,
      app_surface: surface,
    })
  }, [pathname])

  return null
}

export function ProductAnalyticsProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const surface = appSurface(pathname ?? "/")
  const tenant = useTenant()
  const staffHydrated = useStaffStore((state) => state._hydrated)
  const staffToken = useStaffStore((state) => state.token)
  const staffId = useStaffStore((state) => state.staffId)
  const staffRole = useStaffStore((state) => state.role)
  const staffBranchId = useStaffStore((state) => state.branchId)
  const platformHydrated = usePlatformStore((state) => state._hydrated)
  const platformToken = usePlatformStore((state) => state.token)
  const platformUserId = usePlatformStore((state) => state.platformUserId)
  const platformRoles = usePlatformStore((state) => state.roles)

  useEffect(() => {
    ensureInit()
  }, [])

  useEffect(() => {
    if (!tenant.ready) return
    registerTenantProps({ orgId: tenant.organizationId, restaurantSlug: tenant.slug })
  }, [tenant.ready, tenant.organizationId, tenant.slug])

  useEffect(() => {
    if (surface === "guest") {
      activePersona = null
      return
    }

    if (surface === "staff" && staffHydrated) {
      if (staffToken && staffId != null && staffRole && staffBranchId != null) {
        identifyStaff(staffId, staffRole, staffBranchId)
        activePersona = "staff"
      } else if (activePersona === "staff") {
        resetIdentity()
        activePersona = null
      }
    }

    if (surface === "platform" && platformHydrated) {
      if (platformToken && platformUserId != null) {
        identifyPlatform(platformUserId, platformRoles)
        activePersona = "platform"
      } else if (activePersona === "platform") {
        resetIdentity()
        activePersona = null
      }
    }
  }, [
    surface,
    staffHydrated,
    staffToken,
    staffId,
    staffRole,
    staffBranchId,
    platformHydrated,
    platformToken,
    platformUserId,
    platformRoles,
  ])

  useEffect(() => {
    let previous = useWsStore.getState().status
    return useWsStore.subscribe((state) => {
      if (state.status === "failed" && previous !== "failed") {
        track("ws_connection_failed", { attempts: state.attempt })
      }
      previous = state.status
    })
  }, [])

  return (
    <>
      <PageviewTracker
        tenantReady={tenant.ready}
        organizationId={tenant.organizationId}
        restaurantSlug={tenant.slug}
      />
      {children}
    </>
  )
}
