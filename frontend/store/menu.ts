import { create } from "zustand"
import type { MenuCategory } from "@/types/api"

interface MenuState {
  categories: MenuCategory[]
  loading: boolean
  error: string | null

  setCategories: (categories: MenuCategory[]) => void
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  markItemUnavailable: (menuItemId: number) => void
}

export const useMenuStore = create<MenuState>((set) => ({
  categories: [],
  loading: false,
  error: null,

  setCategories(categories) {
    set({ categories, loading: false, error: null })
  },

  setLoading(loading) {
    set({ loading })
  },

  setError(error) {
    set({ error, loading: false })
  },

  markItemUnavailable(menuItemId) {
    set((s) => ({
      categories: s.categories.map((cat) => ({
        ...cat,
        items: cat.items.map((item) =>
          item.id === menuItemId ? { ...item, is_available: false } : item
        ),
      })),
    }))
  },
}))
