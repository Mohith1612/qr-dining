"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { menuApi } from "@/lib/api/menu"
import { analyticsApi, type AnalyticsPeriod, type TopItem, type BusyHour, type OrderVolumeDay } from "@/lib/api/analytics"
import { plansApi, type Subscription, type Plan } from "@/lib/api/plans"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { TopItemsList } from "@/components/analytics/TopItemsList"
import { BusyHoursChart } from "@/components/analytics/BusyHoursChart"
import { OrderVolumeChart } from "@/components/analytics/OrderVolumeChart"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode, Settings2, ChevronDown, ChevronRight, Plus, Trash2, Edit2, Tag, KeyRound, UserX } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, MenuItem, StaffRole, StaffRosterMember, Table, Promo } from "@/types/api"
import { promosApi } from "@/lib/api/promos"
import { tablesApi } from "@/lib/api/tables"
import { themeApi } from "@/lib/api/theme"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { CollateralStudio } from "@/components/collateral/CollateralStudio"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { normalizeCollateralConfig, DEFAULT_COLLATERAL } from "@/types/collateral"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { MenuItemModal } from "@/components/admin/MenuItemModal"
import { StatsSkeleton, TablesSkeleton } from "@/components/shared/LoadingSkeleton"

const PRESET_THEMES = [
  {
    id: "dark-luxury",
    name: "Dark Luxury",
    description: "Warm, intimate, evening dining",
    colors: { bg: "#0E0C09", elev: "#1C1914", accent: "#C9A876", ink: "#F4E8D1" },
  },
  {
    id: "modern-minimal",
    name: "Modern Minimal",
    description: "Airy, daytime, casual bistro",
    colors: { bg: "#FAFAF7", elev: "#F0EDE8", accent: "#16140F", ink: "#16140F" },
  },
] as const

type ThemeId = (typeof PRESET_THEMES)[number]["id"]

export function AppearanceTab() {
  const { branchId, token, role } = useStaffStore()
  const [selectedTheme, setSelectedTheme] = useState<ThemeId>("dark-luxury")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then(b => { if (b.theme) setSelectedTheme(b.theme as ThemeId) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [branchId, token])

  async function handleSave() {
    if (!branchId || !token) return
    setSaving(true)
    try {
      await staffApi.updateBranch(branchId, { theme: selectedTheme }, token)
      document.documentElement.dataset.theme = selectedTheme
      toast.success("Appearance saved")
    } catch {
      toast.error("Failed to save appearance")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Theme</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 18 }}>
          Your guests will see this theme on their phones when they scan the table QR code.
        </p>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          {PRESET_THEMES.map(theme => {
            const isSelected = selectedTheme === theme.id
            return (
              <button
                key={theme.id}
                onClick={() => canEdit && setSelectedTheme(theme.id)}
                disabled={!canEdit}
                style={{
                  width: 160,
                  background: "none",
                  border: `2px solid ${isSelected ? "var(--accent)" : "var(--line-1)"}`,
                  borderRadius: "var(--rad-md)",
                  padding: 0,
                  cursor: canEdit ? "pointer" : "default",
                  overflow: "hidden",
                  boxShadow: isSelected ? "0 0 0 1px var(--accent)" : "none",
                  transition: "border-color 0.15s, box-shadow 0.15s",
                }}
              >
                {/* Color preview area */}
                <div style={{ background: theme.colors.bg, height: 80, padding: 12, display: "flex", flexDirection: "column", gap: 6 }}>
                  <div style={{ display: "flex", gap: 5 }}>
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.elev }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.accent }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.ink, opacity: 0.7 }} />
                  </div>
                  <div style={{ width: "100%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                  <div style={{ width: "70%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                </div>
                {/* Label */}
                <div style={{ background: theme.colors.elev, padding: "8px 10px", textAlign: "left" }}>
                  <p style={{ margin: 0, fontSize: 12, fontWeight: 600, color: theme.colors.ink, lineHeight: 1.3 }}>
                    {theme.name}
                  </p>
                  {isSelected && (
                    <p style={{ margin: "2px 0 0", fontSize: 11, color: theme.colors.accent, lineHeight: 1.2 }}>
                      Selected
                    </p>
                  )}
                </div>
              </button>
            )
          })}
        </div>

        {canEdit && (
          <div style={{ marginTop: 20 }}>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? <Loader2 className="size-4 animate-spin" /> : "Save appearance"}
            </Button>
          </div>
        )}
      </HospitalityCard>
    </div>
  )
}

