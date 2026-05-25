import { create } from "zustand"
import type { MenuCategory, MenuItem } from "@/types/api"

interface MenuState {
  featured: MenuItem[]
  categories: MenuCategory[]
  loading: boolean
  error: string | null

  setMenu: (data: { featured: MenuItem[]; categories: MenuCategory[] }) => void
  setCategories: (categories: MenuCategory[]) => void
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  markItemUnavailable: (menuItemId: number) => void
}

export const useMenuStore = create<MenuState>((set) => ({
  featured: [],
  categories: [],
  loading: false,
  error: null,

  setMenu({ featured, categories }) {
    set({ featured, categories, loading: false, error: null })
  },

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
      featured: s.featured.filter((item) => item.id !== menuItemId),
      categories: s.categories.map((cat) => ({
        ...cat,
        items: cat.items.map((item) =>
          item.id === menuItemId ? { ...item, is_available: false } : item
        ),
      })),
    }))
  },
}))
