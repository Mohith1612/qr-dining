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

function formatFeatureVal(key: keyof Plan["features_json"], val: number | boolean): string {
  if (typeof val === "boolean") return val ? "✓" : "—"
  return val === -1 ? "Unlimited" : String(val)
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleDateString("en", { month: "short", day: "numeric", year: "numeric" })
  } catch {
    return dateStr
  }
}

const PLAN_FEATURES: { label: string; key: keyof Plan["features_json"] }[] = [
  { label: "Branches",     key: "max_branches"  },
  { label: "Tables",       key: "max_tables"     },
  { label: "Analytics",    key: "analytics"      },
  { label: "Multi-branch", key: "multi_branch"   },
]

export function PlanTab() {
  const { branchId, token } = useStaffStore()
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [features, setFeatures] = useState<Plan["features_json"] | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    if (!branchId || !token) {
      setLoading(false)
      return
    }
    setFailed(false)
    // Resolve restaurant_id from the branch (works without a tenant subdomain),
    // then load the subscription for that restaurant.
    staffApi
      .getBranch(branchId, token)
      .then((branch) => plansApi.getSubscription(branch.restaurant_id, token))
      .then((data) => {
        setSubscription(data.subscription)
        setFeatures(data.features)
      })
      .catch(() => {
        setFailed(true)
        toast.error("Couldn't load subscription.")
      })
      .finally(() => setLoading(false))
  }, [branchId, token])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin opacity-40" />
      </div>
    )
  }

  if (failed) {
    return (
      <div className="py-12 text-center">
        <p className="text-[13px] text-[var(--ink-3)]">Couldn&apos;t load your plan. Please try again.</p>
      </div>
    )
  }

  if (!subscription || !features) {
    return <EmptyState icon={CreditCard} title="No subscription found." />
  }

  const isPaid = subscription.plan_tier !== "free"
  const isActive = subscription.status === "active"
  const isTrial = subscription.status === "trial"

  return (
    <div>
      <HospitalityCard elev={2} className="p-5">
        <div className="flex items-center gap-2 flex-wrap mb-1">
          <span className="serif text-lg font-medium text-[var(--ink-1)]">
            {subscription.plan_name}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest border",
              isPaid
                ? "bg-[var(--accent-soft)] text-[var(--accent)] border-[var(--accent)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)] border-[var(--line-2)]"
            )}
          >
            {subscription.plan_tier}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest",
              isActive
                ? "bg-[var(--ok-soft)] text-[var(--ok)]"
                : isTrial
                ? "bg-[var(--warn-soft)] text-[var(--warn)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)]"
            )}
          >
            {subscription.status}
          </span>
        </div>

        {isTrial && subscription.trial_ends_at && (
          <p className="text-[13px] text-[var(--ink-3)] mb-4">
            Trial ends {formatDate(subscription.trial_ends_at)}
          </p>
        )}

        <hr className="rule-strong my-4" />

        <div className="flex flex-col mb-5">
          {PLAN_FEATURES.map(({ label, key }, i) => {
            const val = features[key]
            const inactive = (typeof val === "boolean" && !val) || val === 0
            return (
              <div key={key}>
                {i > 0 && <hr className="rule" />}
                <div className="flex justify-between items-center py-2.5">
                  <span className="text-[13px] text-[var(--ink-3)]">{label}</span>
                  <span className={cn("serif text-[15px] font-medium", inactive ? "text-[var(--ink-4)]" : "text-[var(--ink-1)]")}>
                    {formatFeatureVal(key, val)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>

        <Link
          href="/pricing"
          className="press block text-center py-2.5 rounded-[var(--rad-md)] border border-[var(--line-2)] text-[13px] font-medium text-[var(--ink-2)] no-underline"
        >
          View all plans
        </Link>
      </HospitalityCard>
    </div>
  )
}

