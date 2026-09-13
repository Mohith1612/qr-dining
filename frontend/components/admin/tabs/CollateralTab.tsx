"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { tablesApi } from "@/lib/api/tables"
import { themeApi } from "@/lib/api/theme"
import { CollateralStudio } from "@/components/collateral/CollateralStudio"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { normalizeCollateralConfig, DEFAULT_COLLATERAL } from "@/types/collateral"

export function CollateralTab() {
  const { branchId, token, role } = useStaffStore()
  const { slug } = useTenant()
  const [loading, setLoading] = useState(true)
  const [theme, setTheme] = useState<ThemeConfig | null>(null)
  const [branding, setBranding] = useState<CollateralBranding | null>(null)
  const [tables, setTables] = useState<CollateralTable[]>([])
  const [config, setConfig] = useState<CollateralConfig>(DEFAULT_COLLATERAL)

  const canManage = role === "owner" || role === "manager"

  const load = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const [branch, themeRes, tableList, collateralRes] = await Promise.all([
        staffApi.getBranch(branchId, token),
        themeApi.resolveForBranch(branchId),
        tablesApi.list(branchId, token),
        staffApi.getCollateral(branchId, token),
      ])
      setTheme(themeRes.theme)
      setBranding({
        restaurantName: branch.restaurant_name || branch.name || "Restaurant",
        branchName: branch.name || "",
        logoUrl: branch.logo_url || undefined,
        slug,
      })
      setTables(tableList.map((t) => ({ id: t.id, identifier: t.identifier, qr_code_token: t.qr_code_token })))
      setConfig(normalizeCollateralConfig(collateralRes.collateral))
    } catch {
      toast.error("Couldn't load collateral data.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token, slug])

  useEffect(() => { load() }, [load])

  async function handleSave(next: CollateralConfig) {
    if (!branchId || !token) return false
    try {
      const { collateral } = await staffApi.setCollateral(branchId, next, token)
      setConfig(normalizeCollateralConfig(collateral))
      toast.success("Collateral saved.")
      return true
    } catch (e) {
      if (e instanceof ApiError && e.code === "VALIDATION_ERROR") toast.error(e.message)
      else toast.error("Couldn't save collateral.")
      return false
    }
  }

  if (loading || !branding) {
    return (
      <div className="flex items-center justify-center py-16 text-[var(--ink-3)]">
        <Loader2 className="animate-spin mr-2" size={16} /> Loading collateral…
      </div>
    )
  }

  return (
    <CollateralStudio
      branding={branding}
      theme={theme}
      tables={tables}
      config={config}
      onConfigChange={setConfig}
      onSave={handleSave}
      canManage={canManage}
      mountedFrom="staff_admin"
    />
  )
}
