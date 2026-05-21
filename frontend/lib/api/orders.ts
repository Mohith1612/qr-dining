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
  place: (
    sessionId: string,
    branchId: number,
    participantId: number,
    idempotencyKey: string,
    items: PlaceOrderItem[]
  ) =>
    api.post<PlaceOrderResponse>(`/sessions/${sessionId}/orders`, {
      branch_id: branchId,
      placed_by_participant_id: participantId,
      idempotency_key: idempotencyKey,
      items,
    }),

  list: (sessionId: string) =>
    api.get<Order[]>(`/sessions/${sessionId}/orders`),

  updateStatus: (orderId: string, status: OrderStatus, staffToken: string) =>
    api.patch<Order>(
      `/orders/${orderId}/status`,
      { status },
      { staffToken }
    ),
}
