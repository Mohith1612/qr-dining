"use client"

import { useOrdersStore } from "@/store/orders"
import { ordersApi } from "@/lib/api/orders"
import { generateIdempotencyKey } from "@/lib/idempotency"
import { useSession } from "./useSession"
import type { OrderItem } from "@/types/api"

interface PlaceOrderItem {
  menu_item_id: number
  quantity: number
  modifier_ids?: number[]
  note?: string
}

export function useOrders() {
  const orders = useOrdersStore((s) => s.orders)
  const { session, participant } = useSession()

  const sorted = [...orders].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )

  async function placeOrder(
    items: PlaceOrderItem[],
    promoCode?: string,
    phoneE164?: string
  ): Promise<{ order_items: OrderItem[] } | null> {
    if (!session || !participant) return null
    const key = generateIdempotencyKey()
    const result = await ordersApi.place(
      session.id,
      session.branch_id,
      participant.id,
      key,
      items,
      promoCode,
      phoneE164
    )
    useOrdersStore.getState().addOrder(result.order)
    return result
  }

  return { orders: sorted, placeOrder }
}
