import { api } from "@/lib/api/client"
import type { AnalyticsPeriod } from "@/lib/api/analytics"

// Customer loyalty (entitlement + flag gated; 403 LOYALTY_DISABLED when off).
// Phase 1 redemption is LEDGER-ONLY: staff record a points deduction and apply
// any discount off-system — the bill/payment math is untouched.

export type LoyaltyProgram = {
  organization_id: number
  is_active: boolean
  earn_rate_points: number
  earn_rate_amount: string // decimal string, e.g. "100.00" → X points per ₹100
  configured: boolean
}

export type LoyaltyAccount = {
  account_id: number
  customer_id: number
  points_balance: number
  lifetime_points_earned: number
  lifetime_points_redeemed: number
  visit_count: number
  lifetime_spend: string
}

export type LoyaltyCustomerLookup = {
  customer_id: number
  display_name: string
  phone_e164: string
  account: LoyaltyAccount | null // null = no loyalty activity yet
}

export type LoyaltyTransaction = {
  id: number
  type: "earn" | "redeem" | "adjustment"
  points: number // signed
  amount: string
  payment_id?: number
  session_id?: string
  performed_by_actor_type: "system" | "staff"
  performed_by_staff_id?: number
  reason: string
  created_at: string
}

export type LoyaltyTopCustomer = {
  account_id: number
  customer_id: number
  display_name: string
  phone_e164: string
  points_balance: number
  lifetime_points_earned: number
  visit_count: number
  lifetime_spend: string
}

export type LoyaltyAnalytics = {
  period: string
  points_issued: number
  points_redeemed: number
  earn_count: number
  redeem_count: number
  active_customers: number
  total_accounts: number
  total_sessions: number
  loyalty_sessions: number
  top_customers: LoyaltyTopCustomer[]
}

export const loyaltyApi = {
  getProgram: (branchId: number, staffToken: string) =>
    api.get<LoyaltyProgram>(`/branches/${branchId}/loyalty/program`, { staffToken }),

  putProgram: (
    branchId: number,
    body: { is_active: boolean; earn_rate_points: number; earn_rate_amount: string },
    staffToken: string
  ) => api.put<LoyaltyProgram>(`/branches/${branchId}/loyalty/program`, body, { staffToken }),

  lookupCustomer: (branchId: number, phone: string, staffToken: string) =>
    api.get<LoyaltyCustomerLookup>(
      `/branches/${branchId}/loyalty/customers?phone=${encodeURIComponent(phone)}`,
      { staffToken }
    ),

  listTransactions: (branchId: number, accountId: number, staffToken: string) =>
    api.get<{ transactions: LoyaltyTransaction[] }>(
      `/branches/${branchId}/loyalty/accounts/${accountId}/transactions`,
      { staffToken }
    ),

  redeem: (branchId: number, accountId: number, points: number, reason: string, staffToken: string) =>
    api.post<LoyaltyAccount>(
      `/branches/${branchId}/loyalty/accounts/${accountId}/redeem`,
      { points, reason },
      { staffToken }
    ),

  adjust: (branchId: number, accountId: number, pointsDelta: number, reason: string, staffToken: string) =>
    api.post<LoyaltyAccount>(
      `/branches/${branchId}/loyalty/accounts/${accountId}/adjust`,
      { points_delta: pointsDelta, reason },
      { staffToken }
    ),

  getAnalytics: (branchId: number, period: AnalyticsPeriod, staffToken: string) =>
    api.get<LoyaltyAnalytics>(`/branches/${branchId}/analytics/loyalty?period=${period}`, {
      staffToken,
    }),
}
