"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { Organization } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { PageHeader, PlatformStatusBadge, PlatformLoading } from "@/components/platform/ui"
import { hasPlatformRole } from "@/lib/platform-rbac"
import { ChevronRight, Search, Plus } from "lucide-react"
import { toast } from "sonner"

export default function OrganizationsPage() {
  const token = usePlatformStore((s) => s.token)
  const roles = usePlatformStore((s) => s.roles)
  const canCreate = hasPlatformRole(roles)
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState("")

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    try {
      const { organizations } = await platformApi.listOrganizations(token)
      setOrgs(organizations)
    } catch {
      toast.error("Couldn't load organizations.")
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => { load() }, [load])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return orgs
    return orgs.filter(
      (o) => o.name.toLowerCase().includes(q) || o.code.toLowerCase().includes(q)
    )
  }, [orgs, query])

  return (
    <div>
      <PageHeader
        title="Organizations"
        subtitle={`${orgs.length} total`}
        actions={canCreate ? (
          <Link href="/platform/onboarding">
            <Button><Plus size={15} /> Create organization</Button>
          </Link>
        ) : undefined}
      />

      <div style={{ position: "relative", maxWidth: 360, marginBottom: 16 }}>
        <Search size={15} style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)", color: "var(--ink-4)" }} aria-hidden />
        <Input
          placeholder="Search by name or code"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={{ paddingLeft: 32 }}
        />
      </div>

      {loading ? <PlatformLoading /> : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {filtered.map((o) => (
            <Link key={o.id} href={`/platform/organizations/${o.id}`} style={{ textDecoration: "none" }}>
              <HospitalityCard elev={1} press style={{ padding: "14px 18px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span style={{ fontSize: 15, fontWeight: 600, color: "var(--ink-1)" }}>{o.name}</span>
                    <span className="mono" style={{ fontSize: 12, color: "var(--ink-4)" }}>{o.code}</span>
                  </div>
                  <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                    Created {new Date(o.created_at).toLocaleDateString("en", { year: "numeric", month: "short", day: "numeric" })}
                  </span>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                  <PlatformStatusBadge status={o.status} />
                  <ChevronRight size={16} style={{ color: "var(--ink-4)" }} aria-hidden />
                </div>
              </HospitalityCard>
            </Link>
          ))}
          {filtered.length === 0 && (
            <p style={{ color: "var(--ink-3)", fontSize: 14, padding: "24px 0", textAlign: "center" }}>
              No organizations match.
            </p>
          )}
        </div>
      )}
    </div>
  )
}
