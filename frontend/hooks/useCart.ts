"use client"

import { useCartStore } from "@/store/cart"
import { cartApi } from "@/lib/api/cart"
import { useSession } from "./useSession"
import type { CartItem } from "@/types/api"

export function useCart() {
  const items = useCartStore((s) => s.items)
  const loading = useCartStore((s) => s.loading)
  const { session, participant } = useSession()

  const total = items.reduce((sum, item) => {
    const modifierTotal = item.selected_modifiers?.reduce((s, m) => s + m.price_delta, 0) ?? 0
    return sum + item.quantity * modifierTotal
  }, 0)

  const itemCount = items.reduce((sum, item) => sum + item.quantity, 0)

  async function addItem(
    menuItemId: number,
    quantity: number,
    modifierIds?: number[],
    note?: string
  ): Promise<CartItem | null> {
    if (!session || !participant) return null
    useCartStore.getState().setLoading(true)
    try {
      const item = await cartApi.addItem(session.id, participant.id, menuItemId, quantity, modifierIds, note)
      useCartStore.getState().addItem(item)
      return item
    } finally {
      useCartStore.getState().setLoading(false)
    }
  }

  async function removeItem(itemId: number): Promise<void> {
    if (!session || !participant) return
    const snapshot = useCartStore.getState().items
    useCartStore.getState().removeItem(itemId)
    try {
      await cartApi.removeItem(session.id, participant.id, itemId)
    } catch {
      useCartStore.getState().setItems(snapshot)
    }
  }

  async function refreshCart(): Promise<void> {
    if (!session || !participant) return
    useCartStore.getState().setLoading(true)
    try {
      const fresh = await cartApi.getCart(session.id, participant.id)
      useCartStore.getState().setItems(fresh)
    } finally {
      useCartStore.getState().setLoading(false)
    }
  }

  return { items, loading, total, itemCount, addItem, removeItem, refreshCart }
}
