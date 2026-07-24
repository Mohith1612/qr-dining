import { api } from "./client"
import type {
  PlatformAuthResponse,
  PlatformSession,
  Organization,
  Branch,
  PlatformPlan,
  PlanEntitlement,
  EntitlementCatalogEntry,
  EffectiveEntitlements,
  FeatureFlag,
  FlagState,
  ThemePreset,
  ThemeConfig,
  UsageReport,
  RevenueReport,
  HealthReport,
  SupportSearchResult,
  SupportSessionDetail,
  SupportOrderDetail,
  SupportPaymentDetail,
  AuditEvent,
  OrganizationSubscription,
  BillingProfile,
  SubscriptionInvoice,
  SubscriptionPayment,
  CreateOrgResult,
  CreateBranchResult,
  PlatformTable,
  SubscriptionObservability,
  EntitlementObservability,
  FlagObservability,
  PlatformBranchDetail,
} from "@/types/platform"
import type { CollateralConfig } from "@/types/collateral"
import type {
  WaiterPerformanceRow,
  KitchenPerformanceRow,
  StaffDailyActivityRow,
} from "@/lib/api/analytics"

type Period = "daily" | "weekly" | "monthly"

export interface StaffPerformanceReport {
  branch_id: number
  period: string
  waiters: WaiterPerformanceRow[]
  kitchen: KitchenPerformanceRow[]
  summary: StaffDailyActivityRow[]
}

function orgQuery(period: Period, organizationId?: number): string {
  const p = new URLSearchParams({ period })
  if (organizationId != null) p.set("organization_id", String(organizationId))
  return p.toString()
}

// All authenticated calls pass the platform token via the dedicated client option.
export const platformApi = {
  // ── Auth ──────────────────────────────────────────────────────────────────
  auth: (email: string, password: string, deviceName?: string) =>
    api.post<PlatformAuthResponse>("/platform/auth", {
      email,
      password,
      device_name: deviceName ?? "platform-console",
    }),

  completeMFA: (mfaChallenge: string, mfaCode: string, deviceName?: string) =>
    api.post<PlatformSession>("/platform/auth/mfa", {
      mfa_challenge: mfaChallenge,
      mfa_code: mfaCode,
      device_name: deviceName ?? "platform-console",
    }),

  logout: (token: string) =>
    api.post<void>("/platform/auth/logout", {}, { platformToken: token }),

  // ── Organizations & branches ───────────────────────────────────────────────
  listOrganizations: (token: string) =>
    api.get<{ organizations: Organization[] }>("/platform/organizations", { platformToken: token }),

  getOrganization: (orgId: number, token: string) =>
    api.get<Organization>(`/platform/organizations/${orgId}`, { platformToken: token }),

  // ── Onboarding (operator-driven tenant creation) ────────────────────────────
  createOrganization: (
    body: {
      code: string
      name: string
      legal_name?: string
      primary_contact_email?: string
      restaurant_slug: string
      restaurant_name?: string
    },
    token: string
  ) => api.post<CreateOrgResult>("/platform/organizations", body, { platformToken: token }),

  createBranch: (
    orgId: number,
    body: {
      name: string
      address?: string
      timezone?: string
      branch_code?: string
      order_prefix?: string
      initial_tables?: { identifier: string; capacity?: number }[]
      initial_owner?: { name: string; staff_code: string; pin: string }
    },
    token: string
  ) => api.post<CreateBranchResult>(`/platform/organizations/${orgId}/branches`, body, { platformToken: token }),

  createBranchTables: (
    branchId: number,
    body: { count?: number; capacity?: number; identifiers?: string[] },
    token: string
  ) => api.post<{ tables: PlatformTable[] }>(`/platform/branches/${branchId}/tables`, body, { platformToken: token }),

  suspendOrganization: (orgId: number, token: string) =>
    api.post<Organization>(`/platform/organizations/${orgId}/suspend`, {}, { platformToken: token }),

  activateOrganization: (orgId: number, token: string) =>
    api.post<Organization>(`/platform/organizations/${orgId}/activate`, {}, { platformToken: token }),

  listBranches: (orgId: number, token: string) =>
    api.get<{ branches: Branch[] }>(`/platform/organizations/${orgId}/branches`, { platformToken: token }),

  suspendBranch: (branchId: number, token: string) =>
    api.post<Branch>(`/platform/branches/${branchId}/suspend`, {}, { platformToken: token }),

  activateBranch: (branchId: number, token: string) =>
    api.post<Branch>(`/platform/branches/${branchId}/activate`, {}, { platformToken: token }),

  // ── Plans & entitlements ────────────────────────────────────────────────────
  listEntitlementCatalog: (token: string) =>
    api.get<{ entitlements: EntitlementCatalogEntry[] }>("/platform/entitlements", { platformToken: token }),

  listPlans: (token: string) =>
    api.get<{ plans: PlatformPlan[] }>("/platform/plans", { platformToken: token }),

  createPlan: (
    body: { name: string; tier: string; price_monthly?: string; features?: Record<string, unknown> },
    token: string
  ) => api.post<PlatformPlan>("/platform/plans", body, { platformToken: token }),

  updatePlan: (
    planId: number,
    body: { name?: string; price_monthly?: string; features?: Record<string, unknown> },
    token: string
  ) => api.patch<PlatformPlan>(`/platform/plans/${planId}`, body, { platformToken: token }),

  setPlanEntitlements: (planId: number, entitlements: PlanEntitlement[], token: string) =>
    api.put<{ plan_id: number; entitlements: PlanEntitlement[] }>(
      `/platform/plans/${planId}/entitlements`,
      { entitlements },
      { platformToken: token }
    ),

  getOrganizationEntitlements: (orgId: number, token: string) =>
    api.get<{ entitlements: EffectiveEntitlements }>(
      `/platform/organizations/${orgId}/entitlements`,
      { platformToken: token }
    ),

  // ── Subscription & billing ───────────────────────────────────────────────────
  getSubscription: (orgId: number, token: string) =>
    api.get<{ subscription: OrganizationSubscription | null }>(
      `/platform/organizations/${orgId}/subscription`,
      { platformToken: token }
    ),

  activateSubscription: (
    orgId: number,
    body: { plan_id?: number; expires_at?: string },
    token: string
  ) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/activate`,
      body,
      { platformToken: token }
    ),

  suspendSubscription: (orgId: number, token: string) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/suspend`,
      {},
      { platformToken: token }
    ),

  renewSubscription: (orgId: number, expiresAt: string | undefined, token: string) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/renew`,
      { expires_at: expiresAt },
      { platformToken: token }
    ),

  cancelSubscription: (orgId: number, reason: string, token: string) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/cancel`,
      { reason },
      { platformToken: token }
    ),

  extendTrial: (
    orgId: number,
    body: { plan_id?: number; trial_ends_at: string },
    token: string
  ) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/extend-trial`,
      body,
      { platformToken: token }
    ),

  changeSubscriptionPlan: (orgId: number, planId: number, token: string) =>
    api.post<{ subscription: OrganizationSubscription }>(
      `/platform/organizations/${orgId}/subscription/plan`,
      { plan_id: planId },
      { platformToken: token }
    ),

  getBillingProfile: (orgId: number, token: string) =>
    api.get<{ billing_profile: BillingProfile }>(
      `/platform/organizations/${orgId}/billing-profile`,
      { platformToken: token }
    ),

  updateBillingProfile: (orgId: number, profile: Partial<BillingProfile>, token: string) =>
    api.put<{ billing_profile: BillingProfile }>(
      `/platform/organizations/${orgId}/billing-profile`,
      profile,
      { platformToken: token }
    ),

  listPayments: (orgId: number, token: string) =>
    api.get<{ payments: SubscriptionPayment[] }>(
      `/platform/organizations/${orgId}/payments`,
      { platformToken: token }
    ),

  recordPayment: (
    orgId: number,
    body: {
      method: string
      amount: string
      currency?: string
      reference_number?: string
      notes?: string
      received_at?: string
      invoice_id?: number
    },
    token: string
  ) =>
    api.post<{ payment: SubscriptionPayment }>(
      `/platform/organizations/${orgId}/payments`,
      body,
      { platformToken: token }
    ),

  listInvoices: (orgId: number, token: string) =>
    api.get<{ invoices: SubscriptionInvoice[] }>(
      `/platform/organizations/${orgId}/invoices`,
      { platformToken: token }
    ),

  createInvoice: (
    orgId: number,
    body: { amount: string; currency?: string; due_date?: string; notes?: string },
    token: string
  ) =>
    api.post<{ invoice: SubscriptionInvoice }>(
      `/platform/organizations/${orgId}/invoices`,
      body,
      { platformToken: token }
    ),

  issueInvoice: (orgId: number, invoiceId: number, token: string) =>
    api.post<{ invoice: SubscriptionInvoice }>(
      `/platform/organizations/${orgId}/invoices/${invoiceId}/issue`,
      {},
      { platformToken: token }
    ),

  markInvoicePaid: (orgId: number, invoiceId: number, token: string) =>
    api.post<{ invoice: SubscriptionInvoice }>(
      `/platform/organizations/${orgId}/invoices/${invoiceId}/mark-paid`,
      {},
      { platformToken: token }
    ),

  cancelInvoice: (orgId: number, invoiceId: number, token: string) =>
    api.post<{ invoice: SubscriptionInvoice }>(
      `/platform/organizations/${orgId}/invoices/${invoiceId}/cancel`,
      {},
      { platformToken: token }
    ),

  assignOrganizationPlan: (orgId: number, planId: number, status: string, token: string) =>
    api.put<{ organization_id: number; plan_id: number; status: string }>(
      `/platform/organizations/${orgId}/plan`,
      { plan_id: planId, status },
      { platformToken: token }
    ),

  setEntitlementOverride: (
    orgId: number,
    key: string,
    body: { enabled?: boolean | null; limit_value?: number | null; reason?: string },
    token: string
  ) =>
    api.put<{
      organization_id: number
      entitlement_key: string
      enabled: boolean | null
      limit_value: number | null
      reason: string
    }>(`/platform/organizations/${orgId}/entitlements/${key}`, body, { platformToken: token }),

  // ── Enforcement observability (read-only) ────────────────────────────────────
  getSubscriptionObservability: (token: string) =>
    api.get<SubscriptionObservability>("/platform/observability/subscriptions", { platformToken: token }),

  getEntitlementObservability: (token: string) =>
    api.get<EntitlementObservability>("/platform/observability/entitlements", { platformToken: token }),

  getFlagObservability: (token: string) =>
    api.get<FlagObservability>("/platform/observability/flags", { platformToken: token }),

  // ── Feature flags ────────────────────────────────────────────────────────────
  listFlags: (token: string) =>
    api.get<{ flags: FeatureFlag[] }>("/platform/flags", { platformToken: token }),

  createFlag: (
    body: { key: string; name: string; description?: string; default_enabled?: boolean },
    token: string
  ) => api.post<FeatureFlag>("/platform/flags", body, { platformToken: token }),

  updateFlag: (
    key: string,
    body: { name?: string; description?: string; default_enabled?: boolean },
    token: string
  ) => api.patch<FeatureFlag>(`/platform/flags/${key}`, body, { platformToken: token }),

  setGlobalFlag: (key: string, enabled: boolean, token: string) =>
    api.put<{ flag_key: string; scope: string; enabled: boolean }>(
      `/platform/flags/${key}/global`,
      { enabled },
      { platformToken: token }
    ),

  clearGlobalFlag: (key: string, token: string) =>
    api.delete<void>(`/platform/flags/${key}/global`, { platformToken: token }),

  setOrgFlag: (orgId: number, key: string, enabled: boolean, reason: string, token: string) =>
    api.put<{ flag_key: string; scope: string; organization_id: number; enabled: boolean }>(
      `/platform/organizations/${orgId}/flags/${key}`,
      { enabled, reason },
      { platformToken: token }
    ),

  clearOrgFlag: (orgId: number, key: string, token: string) =>
    api.delete<void>(`/platform/organizations/${orgId}/flags/${key}`, { platformToken: token }),

  setBranchFlag: (branchId: number, key: string, enabled: boolean, reason: string, token: string) =>
    api.put<{ flag_key: string; scope: string; branch_id: number; enabled: boolean }>(
      `/platform/branches/${branchId}/flags/${key}`,
      { enabled, reason },
      { platformToken: token }
    ),

  clearBranchFlag: (branchId: number, key: string, token: string) =>
    api.delete<void>(`/platform/branches/${branchId}/flags/${key}`, { platformToken: token }),

  getOrgFlags: (orgId: number, token: string) =>
    api.get<{ organization_id: number; flags: FlagState[] }>(
      `/platform/organizations/${orgId}/flags`,
      { platformToken: token }
    ),

  getBranchFlags: (branchId: number, token: string) =>
    api.get<{ branch_id: number; flags: FlagState[] }>(
      `/platform/branches/${branchId}/flags`,
      { platformToken: token }
    ),

  // ── Analytics ────────────────────────────────────────────────────────────────
  getUsage: (period: Period, organizationId: number | undefined, token: string) =>
    api.get<UsageReport>(`/platform/analytics/usage?${orgQuery(period, organizationId)}`, { platformToken: token }),

  getRevenue: (period: Period, organizationId: number | undefined, token: string) =>
    api.get<RevenueReport>(`/platform/analytics/revenue?${orgQuery(period, organizationId)}`, { platformToken: token }),

  getHealth: (period: Period, organizationId: number | undefined, token: string) =>
    api.get<HealthReport>(`/platform/analytics/health?${orgQuery(period, organizationId)}`, { platformToken: token }),

  getStaffPerformance: (branchId: number, period: Period, token: string) =>
    api.get<StaffPerformanceReport>(
      `/platform/analytics/staff-performance?branch_id=${branchId}&period=${period}`,
      { platformToken: token }
    ),

  // ── Theme ──────────────────────────────────────────────────────────────────
  listThemePresets: (token: string) =>
    api.get<{ presets: ThemePreset[]; allowed_token_keys: string[] }>(
      "/platform/theme/presets",
      { platformToken: token }
    ),

  getOrganizationTheme: (orgId: number, token: string) =>
    api.get<{ theme: ThemeConfig }>(`/platform/organizations/${orgId}/theme`, { platformToken: token }),

  setOrganizationTheme: (orgId: number, preset: string, tokens: Record<string, string>, token: string) =>
    api.put<{ theme: ThemeConfig }>(
      `/platform/organizations/${orgId}/theme`,
      { preset, tokens },
      { platformToken: token }
    ),

  // ── QR collateral ────────────────────────────────────────────────────────────
  getBranchDetail: (branchId: number, token: string) =>
    api.get<PlatformBranchDetail>(`/platform/branches/${branchId}`, { platformToken: token }),

  listBranchTables: (branchId: number, token: string) =>
    api.get<{ tables: PlatformTable[] }>(`/platform/branches/${branchId}/tables`, { platformToken: token }),

  getBranchCollateral: (branchId: number, token: string) =>
    api.get<{ collateral: CollateralConfig; formats: string[] }>(
      `/platform/branches/${branchId}/collateral`,
      { platformToken: token }
    ),

  setBranchCollateral: (branchId: number, config: CollateralConfig, token: string) =>
    api.put<{ collateral: CollateralConfig }>(
      `/platform/branches/${branchId}/collateral`,
      config,
      { platformToken: token }
    ),

  // ── Support console (read-only) ──────────────────────────────────────────────
  supportSearch: (q: string, token: string) =>
    api.get<{ results: SupportSearchResult[] }>(
      `/platform/support/search?q=${encodeURIComponent(q)}`,
      { platformToken: token }
    ),

  getSupportSession: (sessionId: string, token: string) =>
    api.get<SupportSessionDetail>(`/platform/sessions/${encodeURIComponent(sessionId)}`, { platformToken: token }),

  getSupportOrder: (orderId: string, token: string) =>
    api.get<SupportOrderDetail>(`/platform/orders/${encodeURIComponent(orderId)}`, { platformToken: token }),

  getSupportPayment: (paymentId: string, token: string) =>
    api.get<SupportPaymentDetail>(`/platform/payments/${encodeURIComponent(paymentId)}`, { platformToken: token }),

  listAudit: (
    filters: { organization_id?: number; branch_id?: number; session_id?: string; result?: string; actor_type?: string },
    token: string
  ) => {
    const p = new URLSearchParams()
    if (filters.organization_id != null) p.set("organization_id", String(filters.organization_id))
    if (filters.branch_id != null) p.set("branch_id", String(filters.branch_id))
    if (filters.session_id) p.set("session_id", filters.session_id)
    if (filters.result) p.set("result", filters.result)
    if (filters.actor_type) p.set("actor_type", filters.actor_type)
    const qs = p.toString()
    return api.get<{ audit: AuditEvent[] }>(`/platform/audit${qs ? "?" + qs : ""}`, { platformToken: token })
  },
}
