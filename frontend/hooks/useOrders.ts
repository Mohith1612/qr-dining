"use client"

import { useOrdersStore } from "@/store/orders"
import { ordersApi } from "@/lib/api/orders"
import { generateIdempotencyKey } from "@/lib/idempotency"
import { useSession } from "./useSession"
import type { Order, OrderItem } from "@/types/api"

interface PlaceOrderItem {
  menu_item_id: number
  quantity: number
  modifier_ids?: number[]
  note?: string
}

export function useOrders() {
  const orders = useOrdersStore((s) => s.orders)
  const { session } = useSession()

  const sorted = [...orders].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )

  async function placeOrder(
    items: PlaceOrderItem[]
  ): Promise<{ order: Order; order_items: OrderItem[] } | null> {
    if (!session) return null
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) return null
    const key = generateIdempotencyKey()
    const result = await ordersApi.place(session.id, guestToken, key, items)
    useOrdersStore.getState().addOrder(result.order)
    return result
  }

  return { orders: sorted, placeOrder }
}
