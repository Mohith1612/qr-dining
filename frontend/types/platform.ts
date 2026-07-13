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

// ── Support console (read-only inspection) ───────────────────────────────────

export type SupportSearchResult = {
  type: "organization" | "branch" | "table" | "session" | "participant" | "order" | "payment" | "audit_event"
  id: string
  reference: string
  organization_id: number
  organization_code: string
  branch_id: number
  branch_code: string
  label: string
  status: string
  related_session_id?: string
  related_session_reference?: string
  related_order_id?: string
  related_order_reference?: string
  related_payment_id?: string
  related_payment_reference?: string
  created_at: string
}

export type SupportOrgRef = { id: number; code: string; name: string }
export type SupportBranchRef = { id: number; branch_code: string; name: string; timezone: string }
export type SupportTableRef = { id: number; identifier: string; status: string }

export type SupportSession = {
  id: string
  session_number: string
  status: string
  host_participant_id: number | null
  visit_number: number
  created_at: string
  closed_at: string | null
  awaiting_reactivation_at: string | null
}

export type SupportParticipant = {
  id: number
  display_name: string
  phone_e164?: string
  is_host: boolean
  joined_at: string
  last_seen_at: string
  revoked_at: string | null
  revoked_reason?: string
}

export type SupportOrderItem = {
  id: number
  menu_item_id: number
  quantity: number
  unit_price: string
  selected_modifiers_json?: unknown
  note?: string
}

export type SupportOrder = {
  id: string
  order_operational_id: string
  order_number_display: string
  status: string
  total_amount: string
  discount_amount: string
  created_at: string
  updated_at: string
  items?: SupportOrderItem[]
}

export type SupportPayment = {
  id: number
  payment_reference: string
  status: string
  amount: string
  currency: string
  provider?: string
  provider_payment_ref?: string
  provider_order_ref?: string
  bill_snapshot_id: number | null
  settled_by_staff_id: number | null
  settled_at: string | null
  initiated_at: string
  completed_at: string | null
}

export type SupportAssistance = {
  id: number
  type: string
  status: string
  created_at: string
  resolved_at: string | null
}

export type SupportEvent = {
  id: number
  event_type: string
  actor_type: string
  payload?: unknown
  created_at: string
}

export type SupportBillSnapshot = {
  id: number
  subtotal: string
  discount_amount: string
  tax_amount: string
  service_charge: string
  tip_amount: string
  total: string
  currency: string
  source_order_ids?: unknown
  created_by_actor: string
  created_at: string
}

export type SupportWebhookEvent = {
  id: number
  external_event_id: string
  provider: string
  event_type: string
  processed: boolean
  processed_at: string | null
  error_message?: string
  created_at: string
}

export type SupportSessionDetail = {
  session: SupportSession
  organization: SupportOrgRef
  branch: SupportBranchRef
  table: SupportTableRef
  participants: SupportParticipant[]
  orders: SupportOrder[]
  payments: SupportPayment[]
  assistance: SupportAssistance[]
  timeline: SupportEvent[]
}

export type SupportOrderDetail = {
  order: SupportOrder
  organization: SupportOrgRef
  branch: SupportBranchRef
  session_id: string
  session_number: string
}

export type SupportPaymentDetail = {
  payment: SupportPayment
  organization: SupportOrgRef
  branch: SupportBranchRef
  session_id: string
  session_number: string
  bill_snapshot: SupportBillSnapshot | null
  webhooks: SupportWebhookEvent[]
}

// audit_log row as returned by GET /platform/audit
export type AuditEvent = {
  id: number
  organization_id: number | null
  branch_id: number | null
  session_id: string | null
  resource_type: string
  resource_id: string
  action: string
  result: string
  actor_type: string
  actor_id: string
  actor_display: string
  risk_level: string
  source: string
  ip: string
  event_reference: string
  created_at: string
}
