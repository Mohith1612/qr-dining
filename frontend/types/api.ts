import type { WSEnvelope } from "./ws"

// TypeScript types generated from openapi.yaml — do not diverge from backend contract

export interface APIError {
  code: string
  message: string
  // Optional, additive discriminator sent only where `code` is too coarse to
  // act on (currently: terminal vs retryable guest-credential 401s).
  reason?: string
}

export interface Session {
  id: string
  branch_id: number
  table_id: number
  table_identifier?: string
  session_number?: string
  visit_number?: number
  host_participant_id: number | null
  status: "active" | "closed" | "abandoned" | "awaiting_reactivation" | "expired" | "payment_pending"
  session_token: string
  created_at: string
  closed_at: string | null
}

export interface Participant {
  id: number
  session_id: string
  display_name: string
  is_host: boolean
  joined_at: string
  last_seen_at: string
  device_fingerprint: string | null
}

export interface ModifierSnapshot {
  id: number
  name: string
  price_delta: number
}

export interface CartItem {
  id: number
  session_id: string
  participant_id: number
  menu_item_id: number
  quantity: number
  selected_modifiers: ModifierSnapshot[]
  note: string
  added_at: string
  item_name?: string
  item_price?: number
  image_url?: string | null
}

export interface ItemModifier {
  id: number
  menu_item_id: number
  name: string
  price_delta: number
  is_required?: boolean
  modifier_group?: string
  single_select?: boolean
}

export type DietaryFlag = 'vegetarian' | 'vegan' | 'jain' | 'egg' | 'non-veg'
export type ItemBadge = 'chef-special' | 'bestseller' | 'seasonal' | 'new'

export interface MenuItem {
  id: number
  category_id: number
  branch_id: number
  name: string
  description: string
  price: string
  is_available: boolean
  position: number
  is_featured?: boolean
  featured_sort_order?: number
  modifiers?: ItemModifier[]
  dietary_flags?: DietaryFlag[]
  item_badges?: ItemBadge[]
  spice_level?: number
  image_url?: string | null
}

export interface MenuCategory {
  id: number
  branch_id: number
  name: string
  position: number
  is_active: boolean
  items: MenuItem[]
}

export type OrderStatus =
  | "pending"
  | "confirmed"
  | "preparing"
  | "ready"
  | "served"
  | "cancelled"

export interface Order {
  id: string
  session_id: string
  branch_id: number
  placed_by_participant_id: number
  status: OrderStatus
  total_amount: string
  idempotency_key: string
  created_at: string
  updated_at: string
  order_number?: string | null
  order_number_display?: string
  order_operational_id?: string
}

export interface OrderItem {
  id: number
  order_id: string
  menu_item_id: number
  quantity: number
  unit_price: string
  selected_modifiers: ModifierSnapshot[]
  note: string
}

// KitchenOrderItem is one line on a kitchen ticket, as returned by the enriched
// active-orders payload (GET /branches/:id/orders/active).
export interface KitchenOrderItem {
  menu_item_id: number
  name: string
  quantity: number
  modifiers: ModifierSnapshot[]
  note: string
}

// KitchenOrder is an active order plus the operational item detail the kitchen
// needs to prepare it. Extends Order — every existing order field is preserved.
export interface KitchenOrder extends Order {
  table_identifier?: string
  items: KitchenOrderItem[]
}

export type AssistanceType = "waiter" | "bill" | "other"
export type AssistanceStatus = "pending" | "acknowledged" | "resolved"

export interface AssistanceRequest {
  id: number
  session_id: string
  session_number?: string
  table_id: number
  table_identifier?: string
  participant_id: number
  type: AssistanceType
  status: AssistanceStatus
  created_at: string
}

export type PaymentMethod = "cash" | "card" | "digital" | "card_manual" | "upi"
export type PaymentStatus =
  | "pending"
  | "requested"
  | "provider_pending"
  | "requires_staff_confirmation"
  | "completed"
  | "failed"
  | "cancelled"
  | "refunded"
  | "partially_refunded"

export interface Payment {
  id: number
  session_id: string
  order_id: string | null
  amount: string
  method: PaymentMethod
  status: PaymentStatus
  initiated_at?: string
  created_at?: string
  bill_snapshot_id?: number | null
  branch_id?: number
  currency?: string
  provider?: string | null
  provider_payment_ref?: string | null
  payment_reference?: string
  settled_by_staff_id?: number | null
  settled_at?: string | null
}

// PendingPayment is a payment awaiting staff action for a branch, as returned by
// GET /branches/:id/payments (ListPaymentsForBranchByStatus). `amount` arrives as
// a JSON number from the DB numeric column, so it's typed loosely and coerced.
export interface PendingPayment {
  id: number
  session_id: string
  order_id: string | null
  amount: string | number
  method: PaymentMethod
  status: PaymentStatus
  initiated_at: string
  branch_id: number
  currency: string
  payment_reference: string
  table_identifier: string
  session_number: string
}

export type StaffRole = "owner" | "manager" | "kitchen" | "waiter"

export interface Staff {
  id: number
  branch_id: number
  name: string
  role: StaffRole
  staff_code?: string
}

// Roster row returned by GET /branches/:id/staff (no pin hash; active staff only).
export interface StaffRosterMember {
  id: number
  branch_id: number
  name: string
  role: StaffRole
  staff_code: string
  is_active: boolean
  created_at: string
}

export interface StaffSession {
  staff_id: number
  branch_id: number
  organization_id: number
  role: StaffRole
  token: string
}

export interface SessionSnapshot {
  session: Session
  table_identifier?: string
  participants: Participant[]
  orders: Order[]
  assistance: AssistanceRequest[]
  missed_events?: WSEnvelope[]
  snapshot_at: string
  // When true, this snapshot is the full source of truth and must replace local
  // state wholesale — missed events cannot be replayed contiguously (gap or no
  // incremental basis). See realtime-reconciliation-invariants.
  snapshot_authoritative?: boolean
}

export interface EventLogEntry {
  id: number
  session_id: string
  branch_id: number
  event_type: string
  actor_type: string
  actor_id: number
  payload: Record<string, unknown>
  created_at: string
}

export interface Table {
  id: number
  branch_id: number
  identifier: string
  capacity: number
  qr_code_token: string
  status: 'available' | 'occupied' | 'reserved'
}

export interface Customer {
  id: number
  phone_masked: string
  display_name: string
  visit_count: number
  last_seen_at: string
  opted_in: boolean
}

export interface CustomerHistoryEntry {
  session_id: string
  created_at: string
  closed_at: string | null
  table_identifier: string
  total_spent: string
}

export interface BillItem {
  name: string
  quantity: number
  unit_price: number
  modifiers: string[]
  line_total: number
}

export interface BillOrder {
  order_id: string
  order_number: string
  placed_at: string
  items: BillItem[]
  order_total: number
}

export interface BillData {
  session_id: string
  orders: BillOrder[]
  subtotal: number
  tax_rate: number
  tax_amount: number
  service_charge_rate: number
  service_charge: number
  discount_amount: number
  tip_amount: number
  total: number
  currency: string
}

export interface Promo {
  id: number
  branch_id: number
  code: string
  type: 'flat_amount' | 'percentage'
  value: number
  min_order_amount: number
  max_uses: number | null
  uses_per_phone: number
  valid_from: string
  valid_until: string
  time_window_start: string | null
  time_window_end: string | null
  is_active: boolean
  description: string | null
  created_at: string
}

export interface ValidatePromoResponse {
  promo_id: number
  discount_amount: number
  description: string
}
