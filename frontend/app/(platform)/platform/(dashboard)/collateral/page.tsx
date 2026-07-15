"use client"

import { useCallback, useEffect, useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import { ApiError } from "@/lib/api/client"
import type { Organization, Branch, PlatformBranchDetail } from "@/types/platform"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { normalizeCollateralConfig, DEFAULT_COLLATERAL } from "@/types/collateral"
import { PageHeader, PlatformLoading } from "@/components/platform/ui"
import { CollateralStudio } from "@/components/collateral/CollateralStudio"
import { toast } from "sonner"

// Platform operator QR collateral studio: pick org → branch, load theme + branding +
// tables + saved config, then preview/configure/export premium collateral.
export default function CollateralPage() {
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [orgs, setOrgs] = useState<Organization[]>([])
  const [orgId, setOrgId] = useState<number | undefined>(undefined)
  const [branches, setBranches] = useState<Branch[]>([])
  const [branchId, setBranchId] = useState<number | undefined>(undefined)

  const [theme, setTheme] = useState<ThemeConfig | null>(null)
  const [branding, setBranding] = useState<CollateralBranding | null>(null)
  const [tables, setTables] = useState<CollateralTable[]>([])
  const [config, setConfig] = useState<CollateralConfig>(DEFAULT_COLLATERAL)

  const [loading, setLoading] = useState(true)
  const [loadingBranch, setLoadingBranch] = useState(false)

  useEffect(() => {
    if (!token) return
    platformApi.listOrganizations(token)
      .then((o) => {
        setOrgs(o.organizations)
        setOrgId(o.organizations[0]?.id)
      })
      .catch(() => toast.error("Couldn't load organizations."))
      .finally(() => setLoading(false))
  }, [token])

  useEffect(() => {
    if (!token || !orgId) return
    platformApi.listBranches(orgId, token)
      .then((b) => {
        setBranches(b.branches)
        setBranchId(b.branches[0]?.id)
      })
      .catch(() => toast.error("Couldn't load branches."))
  }, [token, orgId])

  const loadBranch = useCallback(async () => {
    if (!token || !orgId || !branchId) return
    setLoadingBranch(true)
    try {
      const [detail, themeRes, tablesRes, collateralRes] = await Promise.all([
        platformApi.getBranchDetail(branchId, token) as Promise<PlatformBranchDetail>,
        platformApi.getOrganizationTheme(orgId, token),
        platformApi.listBranchTables(branchId, token),
        platformApi.getBranchCollateral(branchId, token),
      ])
      setTheme(themeRes.theme)
      setBranding({
        restaurantName: detail.restaurant_name || detail.name,
        branchName: detail.name,
        logoUrl: detail.logo_url || undefined,
        slug: detail.restaurant_slug || null,
      })
      setTables(tablesRes.tables.map((t) => ({ id: t.id, identifier: t.identifier, qr_code_token: t.qr_code_token })))
      setConfig(normalizeCollateralConfig(collateralRes.collateral))
    } catch {
      toast.error("Couldn't load branch collateral data.")
    } finally {
      setLoadingBranch(false)
    }
  }, [token, orgId, branchId])

  useEffect(() => { loadBranch() }, [loadBranch])

  async function handleSave(next: CollateralConfig) {
    if (!token || !branchId) return
    try {
      const { collateral } = await platformApi.setBranchCollateral(branchId, next, token)
      setConfig(normalizeCollateralConfig(collateral))
      toast.success("Collateral saved.")
    } catch (e) {
      if (e instanceof ApiError && e.code === "VALIDATION_ERROR") toast.error(e.message)
      else if (e instanceof ApiError && e.code === "FORBIDDEN") toast.error("You don't have permission to save collateral.")
      else toast.error("Couldn't save collateral.")
    }
  }

  if (loading) return <PlatformLoading />

  return (
    <div>
      <PageHeader title="QR Collateral" subtitle="Premium, theme-aware table collateral — preview, configure and export print-ready assets" />

      <div style={{ display: "flex", gap: 16, flexWrap: "wrap", marginBottom: 22 }}>
        <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 240 }}>
          <span className="eyebrow">Organization</span>
          <select
            value={orgId ?? ""}
            onChange={(e) => setOrgId(e.target.value ? Number(e.target.value) : undefined)}
            style={selectStyle}
          >
            {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
          </select>
        </label>
        <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 240 }}>
          <span className="eyebrow">Branch</span>
          <select
            value={branchId ?? ""}
            onChange={(e) => setBranchId(e.target.value ? Number(e.target.value) : undefined)}
            style={selectStyle}
            disabled={branches.length === 0}
          >
            {branches.map((b) => <option key={b.id} value={b.id}>{b.name} ({b.branch_code})</option>)}
          </select>
        </label>
      </div>

      {loadingBranch || !branding ? (
        <PlatformLoading />
      ) : (
        <CollateralStudio
          branding={branding}
          theme={theme}
          tables={tables}
          config={config}
          onConfigChange={setConfig}
          onSave={handleSave}
          canManage={canManage}
        />
      )}
    </div>
  )
}

const selectStyle: React.CSSProperties = {
  height: 32,
  borderRadius: "var(--rad-md)",
  border: "1px solid var(--line-2)",
  background: "var(--bg-elev-1)",
  color: "var(--ink-1)",
  padding: "0 10px",
  fontSize: 14,
  fontFamily: "inherit",
}
