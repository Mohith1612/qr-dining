"use client"

import { useMemo } from "react"
import { useMenuStore } from "@/store/menu"
import type { MenuItemView as MenuItem } from "@/types/api-view"

function matchesSearch(item: MenuItem, query: string): boolean {
  const q = query.toLowerCase().trim()
  if (!q) return true
  return (
    item.name.toLowerCase().includes(q) ||
    (item.description?.toLowerCase().includes(q) ?? false)
  )
}

export function useFilteredMenu() {
  const categories = useMenuStore((s) => s.categories)
  const searchQuery = useMenuStore((s) => s.searchQuery)
  const activeFilters = useMenuStore((s) => s.activeFilters)

  return useMemo(() => {
    const hasSearch = searchQuery.trim().length > 0
    const hasFilters =
      activeFilters.dietary.length > 0 ||
      activeFilters.badges.length > 0 ||
      activeFilters.spice !== null

    if (!hasSearch && !hasFilters) return { categories, isFiltered: false }

    const filtered = categories
      .map((cat) => ({
        ...cat,
        items: cat.items.filter((item) => {
          if (hasSearch && !matchesSearch(item, searchQuery)) return false
          if (activeFilters.dietary.length > 0) {
            const match = activeFilters.dietary.some((f) => item.dietary_flags?.includes(f))
            if (!match) return false
          }
          if (activeFilters.badges.length > 0) {
            const match = activeFilters.badges.some((b) => item.item_badges?.includes(b))
            if (!match) return false
          }
          if (activeFilters.spice !== null && (item.spice_level ?? 0) !== activeFilters.spice) {
            return false
          }
          return true
        }),
      }))
      .filter((cat) => cat.items.length > 0)

    return { categories: filtered, isFiltered: true }
  }, [categories, searchQuery, activeFilters])
}
