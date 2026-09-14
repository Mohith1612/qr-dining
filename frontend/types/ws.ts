import type { WebSocketEnvelope, WebSocketEventType } from "./api"

// Live messages carry the full generated envelope. A small number of local
// synthetic terminal events omit transport metadata, so those fields remain
// optional only in the client dispatch type.
export type WSEventType = WebSocketEventType
export type WSEnvelope = Pick<
  WebSocketEnvelope,
  "event" | "session_id" | "timestamp"
> &
  Partial<
    Omit<WebSocketEnvelope, "event" | "session_id" | "timestamp">
  >

export type WSEventHandler<T = unknown> = (payload: T, envelope: WSEnvelope) => void

export type WSEventHandlerMap = Partial<Record<WSEventType, WSEventHandler>>
