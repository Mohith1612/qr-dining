"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { useBrandingStore } from "@/store/branding"
import { staffApi } from "@/lib/api/staff"
import { StaffBar } from "@/components/staff/StaffBar"

export default function StaffLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { token, branchId, _hydrated, clear } = useStaffStore()

  useEffect(() => {
    if (_hydrated && !token) {
      router.replace("/staff/login")
    }
  }, [token, _hydrated, router])

  // Resolve the tenant's name + logo for the header (branch-scoped, so it works
  // in local multi-tenant testing where there's no per-tenant subdomain).
  useEffect(() => {
    if (!token || !branchId) return
    staffApi.getBranch(branchId, token)
      .then(b => useBrandingStore.getState().setBranding({ logoUrl: b.logo_url, name: b.restaurant_name }))
      .catch(() => {})
  }, [token, branchId])

  // Staff is a fixed "ops" surface, not tenant-branded. Set it on <html> too so
  // portaled toasts/sheets inherit ops tokens; clear on unmount.
  useEffect(() => {
    document.documentElement.setAttribute("data-surface", "ops")
    return () => document.documentElement.removeAttribute("data-surface")
  }, [])

  if (!_hydrated || !token) return null

  async function handleSignOut() {
    if (token) {
      try { await staffApi.logout(token) } catch {}
    }
    clear()
    router.replace("/staff/login")
  }

  return (
    <div data-surface="ops" className="min-h-screen flex flex-col" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      <StaffBar onSignOut={handleSignOut} />
      <main className="flex-1">{children}</main>
    </div>
  )
}
