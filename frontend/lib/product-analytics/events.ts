import type { AssistanceType, OrderStatus, PaymentMethod, StaffRole } from "@/types/api"
import { ensureInit } from "@/lib/product-analytics/posthog"

type Id = string | number
type ErrorProps = { error_code: string }

export interface EventPropsMap {
  qr_resolved: { branch_id: number; table_id: number; has_active_session: boolean }
  qr_resolve_failed: ErrorProps
  session_created: { session_id: string; participant_id: number; phone_provided: boolean }
  session_joined: { session_id: string; participant_id: number; phone_provided: boolean }
  item_viewed: { item_id: number; item_name: string; category_id: number }
  item_added: { item_id: number; item_name: string; quantity: number; unit_price: number; modifiers_count: number }
  cart_item_removed: { item_id: number; quantity: number }
  order_placed: { order_id: string; item_count: number; subtotal: number }
  promo_applied: { promo_code: string; discount_amount: number }
  promo_apply_failed: { promo_code: string; error_code: string }
  promo_removed: { promo_code: string }
  payment_initiated: { method: PaymentMethod; amount: number }
  payment_completed: { method: PaymentMethod; amount: number }
  assistance_requested: { assistance_type: AssistanceType }
  host_transferred: { to_participant_id: number }
  guest_optin_submitted: { has_name: boolean }
  session_ended: { session_id: string }
  $pageview: { $current_url: string; $pathname: string; route: string; app_surface: string }
  staff_login_succeeded: { role: StaffRole; branch_id: number }
  staff_login_failed: ErrorProps
  staff_logout: Record<string, never>
  order_advanced: { order_id: string; from_status: OrderStatus; to_status: OrderStatus }
  order_cancelled: { order_id: string; from_status: OrderStatus }
  order_served: { order_id: string }
  payment_settled: { session_id: string; method: PaymentMethod; amount: number }
  payment_cancelled_by_staff: { session_id: string; payment_id: number; role: StaffRole }
  session_force_closed: { session_id: string; cancelled_payment_count: number; role: StaffRole }
  assistance_acknowledged: { request_id: number; assistance_type: AssistanceType; seconds_to_ack?: number }
  admin_tab_viewed: { tab: string }
  upsell_gate_viewed: { feature: "staff_performance" | "loyalty" }
  collateral_saved: { template_id: Id; mounted_from: "staff_admin" | "platform" }
  collateral_printed: { kind: string; mounted_from: "staff_admin" | "platform" }
  collateral_exported: { kind: string; mounted_from: "staff_admin" | "platform" }
  platform_login_succeeded: { mfa_used: boolean }
  platform_login_failed: { error_code: string; stage: "credentials" | "mfa" }
  platform_logout: Record<string, never>
  ws_connection_failed: { attempts: number }
  app_error: { message: string; route: string }
}

export type AnalyticsEvent = keyof EventPropsMap

export function track<E extends AnalyticsEvent>(name: E, props: EventPropsMap[E]): void {
  ensureInit()?.capture(name, props)
}
