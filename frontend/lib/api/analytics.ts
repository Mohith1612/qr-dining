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
}
