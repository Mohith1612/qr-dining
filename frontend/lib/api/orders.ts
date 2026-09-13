import { api } from "./client"
import type { Order, OrderItem, OrderStatus } from "@/types/api"

interface PlaceOrderItem {
  menu_item_id: number
  quantity: number
  modifier_ids?: number[]
  note?: string
}

interface PlaceOrderResponse {
  order: Order
  order_items: OrderItem[]
}

export const ordersApi = {
  // Promos are applied at the bill now, not at order placement.
  place: (
    sessionId: string,
    guestToken: string,
    idempotencyKey: string,
    items: PlaceOrderItem[]
  ) =>
    api.post<PlaceOrderResponse>(`/sessions/${sessionId}/orders`, {
      idempotency_key: idempotencyKey,
      items,
    }, { guestToken }),

  list: (sessionId: string) =>
    api.get<Order[]>(`/sessions/${sessionId}/orders`),

  updateStatus: (orderId: string, status: OrderStatus, staffToken: string) =>
    api.patch<Order>(
      `/orders/${orderId}/status`,
      { status },
      { staffToken }
    ),
}
