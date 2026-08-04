"use client"

import { useCallback, useEffect, useState } from "react"
import { useStaffStore } from "@/store/staff"
import {
  analyticsApi,
  type AnalyticsPeriod,
  type WaiterPerformanceRow,
  type KitchenPerformanceRow,
  type StaffDailyActivityRow,
} from "@/lib/api/analytics"
import { ApiError } from "@/lib/api/client"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { EmptyState } from "@/components/shared/EmptyState"
import { StatsSkeleton } from "@/components/shared/LoadingSkeleton"
import { StaffPerformanceTables } from "@/components/staff-performance/StaffPerformanceTables"
import { Activity, Lock } from "lucide-react"
import { toast } from "sonner"
import { track } from "@/lib/product-analytics/events"

// Manager-facing staff performance. Derived from existing operational events;
// "sessions" means sessions the staff member touched (no assignment system).
export function PerformanceTab() {
  const { branchId, token, role } = useStaffStore()
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [waiters, setWaiters] = useState<WaiterPerformanceRow[]>([])
  const [kitchen, setKitchen] = useState<KitchenPerformanceRow[]>([])
  const [summary, setSummary] = useState<StaffDailyActivityRow[]>([])
  const [gated, setGated] = useState(false)
  const [loading, setLoading] = useState(true)

  const canView = role === "owner" || role === "manager"

  const fetchAll = useCallback(async () => {
    if (!branchId || !token || !canView) return
    setLoading(true)
    setGated(false)
    try {
      const [w, k, s] = await Promise.all([
        analyticsApi.getWaiterPerformance(branchId, period, token),
        analyticsApi.getKitchenPerformance(branchId, period, token),
        analyticsApi.getStaffSummary(branchId, period, token),
      ])
      setWaiters(w.waiters)
      setKitchen(k.kitchen)
      setSummary(s.summary)
    } catch (err) {
      if (err instanceof ApiError && err.code === "STAFF_ANALYTICS_DISABLED") {
        track("upsell_gate_viewed", { feature: "staff_performance" })
        setGated(true)
      } else {
        toast.error("Couldn't load staff performance.")
      }
    } finally {
      setLoading(false)
    }
  }, [branchId, token, period, canView])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  if (!canView) {
    return (
      <EmptyState
        icon={Lock}
        title="Owner or manager only"
        description="Staff performance is visible to owners and managers."
      />
    )
  }

  if (loading) return <StatsSkeleton />

  if (gated) {
    return (
      <EmptyState
        icon={Activity}
        title="Staff performance not enabled"
        description="Staff performance analytics is not enabled for this organization. Ask your platform operator to enable it."
      />
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <PeriodSelector value={period} onChange={setPeriod} />
      <StaffPerformanceTables waiters={waiters} kitchen={kitchen} summary={summary} />
    </div>
  )
}
