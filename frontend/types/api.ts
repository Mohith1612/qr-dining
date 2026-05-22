// TypeScript types generated from openapi.yaml — do not diverge from backend contract

export interface APIError {
  code: string
  message: string
}

export interface Session {
  id: string
  branch_id: number
  table_id: number
  table_identifier?: string
  host_participant_id: number | null
  status: "active" | "closed" | "abandoned"
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
}

export interface ItemModifier {
  id: number
  menu_item_id: number
  name: string
  price_delta: number
}

export interface MenuItem {
  id: number
  category_id: number
  branch_id: number
  name: string
  description: string
  price: string
  is_available: boolean
  position: number
  modifiers?: ItemModifier[]
}

export interface MenuCategory {
  id: number
  branch_id: number
  name: string
  position: number
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

export type AssistanceType = "waiter" | "bill" | "other"
export type AssistanceStatus = "pending" | "acknowledged" | "resolved"

export interface AssistanceRequest {
  id: number
  session_id: string
  table_id: number
  participant_id: number
  type: AssistanceType
  status: AssistanceStatus
  created_at: string
}

export type PaymentMethod = "cash" | "card" | "digital"
export type PaymentStatus = "pending" | "completed" | "failed" | "refunded"

export interface Payment {
  id: number
  session_id: string
  order_id: string | null
  amount: string
  method: PaymentMethod
  status: PaymentStatus
  created_at: string
}

export type StaffRole = "owner" | "manager" | "kitchen" | "waiter"

export interface Staff {
  id: number
  branch_id: number
  name: string
  role: StaffRole
}

export interface StaffSession {
  staff_id: number
  branch_id: number
  role: StaffRole
  token: string
}

export interface SessionSnapshot {
  session: Session
  table_identifier?: string
  participants: Participant[]
  orders: Order[]
  assistance: AssistanceRequest[]
  snapshot_at: string
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
