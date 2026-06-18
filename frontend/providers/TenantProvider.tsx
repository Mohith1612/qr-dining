"use client"

import { createContext, useContext, useEffect, useState } from "react"
import { env } from "@/config/env"

export type TenantSettings = {
  theme?: string
  [key: string]: unknown
}

export type TenantContext = {
  restaurantId: number | null
  organizationId: number | null
  name: string | null
  slug: string | null
  settings: TenantSettings
  ready: boolean
}

const TenantCtx = createContext<TenantContext>({
  restaurantId: null,
  organizationId: null,
  name: null,
  slug: null,
  settings: {},
  ready: false,
})

// Resolve the tenant slug: env override for local dev, or parse from hostname.
function resolveTenantSlug(): string | null {
  if (env.tenantSlug) return env.tenantSlug

  if (typeof window === "undefined") return null

  const hostname = window.location.hostname
  const baseDomain = env.baseDomain
  if (!baseDomain) return null

  const suffix = `.${baseDomain}`
  if (!hostname.endsWith(suffix)) return null

  const slug = hostname.slice(0, -suffix.length)
  if (slug.includes(".") || !slug) return null

  return slug
}

export function TenantProvider({ children }: { children: React.ReactNode }) {
  const [tenant, setTenant] = useState<TenantContext>({
    restaurantId: null,
    organizationId: null,
    name: null,
    slug: null,
    settings: {},
    ready: false,
  })

  useEffect(() => {
    const slug = resolveTenantSlug()
    if (!slug) {
      setTenant(t => ({ ...t, ready: true }))
      return
    }

    fetch(`${env.apiUrl}/tenants/by-slug/${encodeURIComponent(slug)}`)
      .then(r => {
        if (!r.ok) throw new Error(`tenant not found: ${slug}`)
        return r.json()
      })
      .then(data => {
        setTenant({
          restaurantId: data.id,
          organizationId: data.organization_id ?? null,
          name: data.name,
          slug: data.slug,
          settings: data.settings ?? {},
          ready: true,
        })
      })
      .catch(err => {
        console.warn("[TenantProvider] failed to resolve tenant:", err)
        setTenant(t => ({ ...t, ready: true }))
      })
  }, [])

  return <TenantCtx.Provider value={tenant}>{children}</TenantCtx.Provider>
}

export function useTenant(): TenantContext {
  return useContext(TenantCtx)
}
