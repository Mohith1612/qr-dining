import { api } from "./client"
import type { Order, OrderItem, OrderStatus } from "@/types/api"

interface PlaceOrderItem {
  menu_item_id: number
  quantity: number
  modifier_ids?: number[]
  note?: string
}

interface BackendPlaceOrderResponse {
  Order: Order
  OrderItems: OrderItem[]
}

interface PlaceOrderResponse {
  order: Order
  order_items: OrderItem[]
}

export const ordersApi = {
  place: (
    sessionId: string,
    _branchId: number,
    participantId: number,
    idempotencyKey: string,
    items: PlaceOrderItem[],
    promoCode?: string,
    phoneE164?: string
  ) =>
    api.post<BackendPlaceOrderResponse>(`/sessions/${sessionId}/orders`, {
      idempotency_key: idempotencyKey,
      items,
      ...(promoCode ? { promo_code: promoCode } : {}),
      ...(phoneE164 ? { phone_e164: phoneE164 } : {}),
    }, { participantId }).then(r => ({ order: r.Order, order_items: r.OrderItems }) as PlaceOrderResponse),

  list: (sessionId: string) =>
    api.get<Order[]>(`/sessions/${sessionId}/orders`),

  updateStatus: (orderId: string, status: OrderStatus, staffToken: string) =>
    api.patch<Order>(
      `/orders/${orderId}/status`,
      { status },
      { staffToken }
    ),
}
