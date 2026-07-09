// Platform control-plane types. Mirror the backend contracts on branch
// platform-governance-entitlements. The platform layer is a separate trust domain.

export type PlatformRole =
  | "super_admin"
  | "support_admin"
  | "billing_admin"
  | "read_only_auditor"

export type PlatformSession = {
  token: string
  session_id: string
  platform_user_id: number
  email: string
  display_name: string
  roles: PlatformRole[]
  expires_at: string
}

export type PlatformMFAChallenge = {
  mfa_required: true
  mfa_challenge: string
  mfa_expires_at: string
}

export type PlatformAuthResponse = PlatformSession | PlatformMFAChallenge

export function isMFAChallenge(r: PlatformAuthResponse): r is PlatformMFAChallenge {
  return (r as PlatformMFAChallenge).mfa_required === true
}

export type OrgStatus = "active" | "suspended" | "archived"

export type Organization = {
  id: number
  code: string
  name: string
  legal_name: string
  primary_contact_email: string
  status: OrgStatus
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type Branch = {
  id: number
  restaurant_id: number
  organization_id: number
  name: string
  address: string
  timezone: string
  branch_code: string
  status: OrgStatus
  support_metadata: Record<string, unknown>
  session_timeout_minutes: number
  order_prefix: string
  created_at: string
}

export type PlanTier = "free" | "standard" | "premium"

export type PlanEntitlement = {
  key: string
  enabled: boolean
  limit_value: number | null
}

export type PlatformPlan = {
  id: number
  name: string
  tier: PlanTier
  price_monthly: string
  features_json: Record<string, unknown>
  entitlements: PlanEntitlement[]
}

export type EntitlementCatalogEntry = {
  key: string
  kind: "capability" | "limit"
  description: string
}

export type EffectiveEntitlements = {
  organization_id: number
  plan_tier: string
  capabilities: Record<string, boolean>
  limits: Record<string, number> // -1 = unlimited
  source: "org_assignment" | "restaurant_bridge" | "free_default"
}

export type FeatureFlag = {
  key: string
  name: string
  description: string
  default_enabled: boolean
  global_override: boolean | null
}

export type FlagSource =
  | "branch_override"
  | "org_override"
  | "global_override"
  | "default"

export type FlagState = {
  key: string
  enabled: boolean
  source: FlagSource
}

export type ThemePreset = {
  key: string
  name: string
  description: string
}

export type ThemeConfig = {
  preset: string
  tokens: Record<string, string>
}

// Analytics
export type DayCount = { day: string; count: number }
export type DayRevenue = { day: string; count: number; revenue: string }
export type BranchRevenue = {
  branch_id: number
  branch_name: string
  branch_code: string
  count: number
  revenue: string
}
export type OrgRevenue = {
  organization_id: number
  organization_code: string
  count: number
  revenue: string
}

export type UsageReport = {
  period: string
  sessions_per_day: DayCount[]
  orders_per_day: DayCount[]
  payments_per_day: DayCount[]
  participant_joins_per_day: DayCount[]
  active_branches: number
  active_diners: number
}

export type RevenueReport = {
  period: string
  gmv: string
  revenue_per_day: DayRevenue[]
  revenue_by_branch: BranchRevenue[]
  revenue_by_org: OrgRevenue[]
}

export type HealthReport = {
  period: string
  authz_denials_per_day: DayCount[]
  webhook_failures_per_day: DayCount[]
}
