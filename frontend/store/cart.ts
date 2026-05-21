import { create } from "zustand"
import type { CartItem } from "@/types/api"

interface CartState {
  items: CartItem[]
  loading: boolean

  setItems: (items: CartItem[]) => void
  addItem: (item: CartItem) => void
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
