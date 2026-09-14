"use client"

import { useCartStore } from "@/store/cart"
import { cartApi } from "@/lib/api/cart"
import { useSession } from "./useSession"
import type { CartViewItem } from "@/types/api-view"

export function useCart() {
  const items = useCartStore((s) => s.items)
  const loading = useCartStore((s) => s.loading)
  const { session } = useSession()

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
  ): Promise<CartViewItem | null> {
    if (!session) return null
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) return null
    useCartStore.getState().setLoading(true)
    try {
      const item = await cartApi.addItem(session.id, guestToken, menuItemId, quantity, modifierIds, note)
      useCartStore.getState().addItem(item)
      return item
    } finally {
      useCartStore.getState().setLoading(false)
    }
  }

  async function removeItem(itemId: number): Promise<boolean> {
    if (!session) return false
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) return false
    const snapshot = useCartStore.getState().items
    useCartStore.getState().removeItem(itemId)
    try {
      await cartApi.removeItem(session.id, guestToken, itemId)
      return true
    } catch (error) {
      useCartStore.getState().setItems(snapshot)
      throw error
    }
  }

  async function refreshCart(): Promise<void> {
    if (!session) return
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) return
    useCartStore.getState().setLoading(true)
    try {
      const fresh = await cartApi.getCart(session.id, guestToken)
      useCartStore.getState().setItems(fresh)
    } finally {
      useCartStore.getState().setLoading(false)
    }
  }

  return { items, loading, total, itemCount, addItem, removeItem, refreshCart }
}
