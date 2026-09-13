import { api } from "@/lib/api/client"

export type AnalyticsPeriod = "daily" | "weekly" | "monthly"

export type TopItem = {
  menu_item_id: number
  menu_item_name: string
  total_quantity: number
}

export type BusyHour = {
  hour: number
  order_count: number
}

export type OrderVolumeDay = {
  day: string // "YYYY-MM-DD"
  order_count: number
  revenue: string // decimal string e.g. "1234.50"
}

// Staff performance (entitlement + flag gated; 403 STAFF_ANALYTICS_DISABLED when off).
// "sessions_handled" means distinct sessions the staff member touched (served /
// settled / assisted) — the system has no waiter↔session assignment.
export type WaiterPerformanceRow = {
  staff_id: number
  staff_name: string
  staff_role: string
  sessions_handled: number
  tables_served: number
  orders_assisted: number
  orders_served: number
  assistance_accepted: number
  assistance_resolved: number
  avg_response_seconds: number
  payments_settled: number
  avg_settlement_seconds: number
  login_count: number
  active_seconds: number
}

export type KitchenPerformanceRow = {
  staff_id: number
  staff_name: string
  orders_completed: number
  avg_prep_seconds: number
  peak_orders_per_hour: number
}

export type StaffDailyActivityRow = {
  staff_id: number
  staff_name: string
  staff_role: string
  day: string // "YYYY-MM-DD" (UTC)
  event_count: number
  login_count: number
  active_seconds: number
}

export const analyticsApi = {
  getTopItems: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; items: TopItem[] }>(
      `/branches/${branchId}/analytics/top-items?period=${period}`,
      { staffToken }
    ),

  getBusyHours: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; hours: BusyHour[] }>(
      `/branches/${branchId}/analytics/busy-hours?period=${period}`,
      { staffToken }
    ),

  getOrderVolume: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; days: OrderVolumeDay[] }>(
      `/branches/${branchId}/analytics/order-volume?period=${period}`,
      { staffToken }
    ),

  getWaiterPerformance: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; waiters: WaiterPerformanceRow[] }>(
      `/branches/${branchId}/analytics/staff/waiters?period=${period}`,
      { staffToken }
    ),

  getKitchenPerformance: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; kitchen: KitchenPerformanceRow[] }>(
      `/branches/${branchId}/analytics/staff/kitchen?period=${period}`,
      { staffToken }
    ),

  getStaffSummary: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<{ period: string; summary: StaffDailyActivityRow[] }>(
      `/branches/${branchId}/analytics/staff/summary?period=${period}`,
      { staffToken }
    ),
}
