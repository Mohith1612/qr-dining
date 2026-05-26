import { create } from "zustand"
import type { MenuCategory, MenuItem, DietaryFlag, ItemBadge } from "@/types/api"

interface ActiveFilters {
  dietary: DietaryFlag[]
  badges: ItemBadge[]
  spice: number | null
}

interface MenuState {
  featured: MenuItem[]
  categories: MenuCategory[]
  loading: boolean
  error: string | null
  activeFilters: ActiveFilters
  searchQuery: string
  isSearchMode: boolean

  setMenu: (data: { featured: MenuItem[]; categories: MenuCategory[] }) => void
  setCategories: (categories: MenuCategory[]) => void
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  markItemUnavailable: (menuItemId: number) => void
  setActiveFilters: (f: Partial<ActiveFilters>) => void
  clearFilters: () => void
  setSearchQuery: (q: string) => void
  setSearchMode: (active: boolean) => void
}

const defaultFilters: ActiveFilters = { dietary: [], badges: [], spice: null }

export const useMenuStore = create<MenuState>((set) => ({
  featured: [],
  categories: [],
  loading: false,
  error: null,
  activeFilters: defaultFilters,
  searchQuery: "",
  isSearchMode: false,

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

  setActiveFilters(f) {
    set((s) => ({ activeFilters: { ...s.activeFilters, ...f } }))
  },

  clearFilters() {
    set({ activeFilters: defaultFilters })
  },

  setSearchQuery(q) {
    set({ searchQuery: q })
  },

  setSearchMode(active) {
    set({ isSearchMode: active })
  },
}))
