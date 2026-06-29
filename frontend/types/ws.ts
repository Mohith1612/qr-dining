// WebSocket types — matches internal/websocket/message.go exactly
// Field name is "event" (not "type"), values are SCREAMING_SNAKE_CASE
// Payload field is "payload" (not "data")

export type WSEventType =
  | "SESSION_CREATED"
  | "SESSION_CLOSED"
  | "SESSION_EXPIRING_SOON"
  | "SESSION_REACTIVATED"
  | "PARTICIPANT_JOINED"
  | "PARTICIPANT_LEFT"
  | "HOST_CHANGED"
  | "CART_UPDATED"
  | "ORDER_PLACED"
  | "ORDER_CONFIRMED"
  | "ORDER_PREPARING"
  | "ORDER_READY"
  | "ORDER_SERVED"
  | "ORDER_CANCELLED"
  | "ASSISTANCE_REQUESTED"
  | "ASSISTANCE_ACKNOWLEDGED"
  | "ASSISTANCE_RESOLVED"
  | "PAYMENT_INITIATED"
  | "PAYMENT_COMPLETED"
  | "MENU_ITEM_AVAILABILITY_CHANGED"
  | "PING"
  | "PONG"

export interface WSEnvelope {
  event_id?: string
  sequence?: number
  organization_id?: number
  branch_id?: number
  event: WSEventType
  session_id: string
  payload: unknown
  timestamp: string
}

export type WSEventHandler<T = unknown> = (payload: T, envelope: WSEnvelope) => void

export type WSEventHandlerMap = Partial<Record<WSEventType, WSEventHandler>>
