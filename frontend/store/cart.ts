import { create } from "zustand"
import type { CartViewItem } from "@/types/api-view"

interface CartState {
  items: CartViewItem[]
  loading: boolean

  setItems: (items: CartViewItem[]) => void
  addItem: (item: CartViewItem) => void
  removeItem: (itemId: number) => void
  setLoading: (loading: boolean) => void
  clear: () => void
}

export const useCartStore = create<CartState>((set) => ({
  items: [],
  loading: false,

  setItems(items) {
    set({ items, loading: false })
  },

  addItem(item) {
    set((s) => ({ items: [...s.items, item] }))
  },

  removeItem(itemId) {
    set((s) => ({ items: s.items.filter((i) => i.id !== itemId) }))
  },

  setLoading(loading) {
    set({ loading })
  },

  clear() {
    set({ items: [] })
  },
}))
