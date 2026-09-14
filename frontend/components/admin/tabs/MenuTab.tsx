"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { formatCurrency } from "@/lib/format"
import { Loader2, MoreVertical, ChevronDown, ChevronRight, Plus, Trash2, Edit2 } from "lucide-react"
import { toast } from "sonner"
import type { MenuCategoryView as MenuCategory, MenuItemView as MenuItem } from "@/types/api-view"
import { MenuItemModal } from "@/components/admin/MenuItemModal"

export function MenuTab() {
  const { branchId, token, role } = useStaffStore()
  const canManage = role === "owner" || role === "manager"

  const [categories, setCategories] = useState<MenuCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  // Per-item states
  const [toggling, setToggling] = useState<number | null>(null)
  const [bulkToggling, setBulkToggling] = useState<number | null>(null) // category ID

  // Unified item modal
  const [modalState, setModalState] = useState<{
    open: boolean
    mode: "create" | "edit"
    item?: MenuItem
    defaultCategoryId?: number
  }>({ open: false, mode: "create" })

  // Category management
  const [catMore, setCatMore] = useState<number | null>(null) // category ID with open dropdown
  const [renaming, setRenaming] = useState<number | null>(null)
  const [renameValue, setRenameValue] = useState("")

  // Add category
  const [addingCat, setAddingCat] = useState(false)
  const [newCatName, setNewCatName] = useState("")
  const [creatingCat, setCreatingCat] = useState(false)

  const loadMenu = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await staffApi.getAdminMenu(branchId, token)
      setCategories(data.categories)
      setExpanded(new Set(data.categories.map((c) => c.id)))
    } catch {
      toast.error("Couldn't load menu.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => { loadMenu() }, [loadMenu])

  function updateItemInState(itemId: number, patch: Partial<MenuItem>) {
    setCategories((prev) =>
      prev.map((cat) => ({
        ...cat,
        items: cat.items.map((item) => item.id === itemId ? { ...item, ...patch } : item),
      }))
    )
  }

  async function handleToggle(itemId: number, current: boolean) {
    if (!branchId || !token) return
    setToggling(itemId)
    const next = !current
    updateItemInState(itemId, { is_available: next })
    try {
      await staffApi.toggleAvailability(itemId, branchId, next, token)
    } catch {
      updateItemInState(itemId, { is_available: current })
      toast.error("Couldn't update availability.")
    } finally {
      setToggling(null)
    }
  }

  async function handleBulkToggle(catId: number, available: boolean) {
    if (!branchId || !token) return
    const cat = categories.find((c) => c.id === catId)
    if (!cat) return
    setBulkToggling(catId)
    setCatMore(null)
    try {
      for (const item of cat.items) {
        await staffApi.toggleAvailability(item.id, branchId, available, token)
        updateItemInState(item.id, { is_available: available })
      }
    } catch {
      toast.error("Some items couldn't be updated.")
    } finally {
      setBulkToggling(null)
    }
  }

  async function handleDeleteItem(itemId: number, catId: number) {
    if (!branchId || !token) return
    if (!confirm("Delete this item?")) return
    try {
      await staffApi.deleteMenuItem(itemId, branchId, token)
      setCategories((prev) =>
        prev.map((cat) =>
          cat.id === catId ? { ...cat, items: cat.items.filter((i) => i.id !== itemId) } : cat
        )
      )
    } catch {
      toast.error("Couldn't delete item.")
    }
  }

  async function handleDeleteCategory(catId: number) {
    if (!branchId || !token) return
    if (!confirm("Delete this category?")) return
    setCatMore(null)
    try {
      await staffApi.deleteCategory(catId, branchId, token)
      setCategories((prev) => prev.filter((c) => c.id !== catId))
    } catch (err: unknown) {
      if (err && typeof err === "object" && "code" in err && (err as { code: string }).code === "CATEGORY_NOT_EMPTY") {
        toast.error("Remove all items from this category before deleting it.")
      } else {
        toast.error("Couldn't delete category.")
      }
    }
  }

  async function handleRenameCategory(catId: number) {
    if (!branchId || !token) return
    const cat = categories.find((c) => c.id === catId)
    if (!cat || !renameValue.trim()) return
    try {
      const updated = await staffApi.updateCategory(catId, branchId, {
        name: renameValue.trim(),
        position: cat.position,
        is_active: cat.is_active,
      }, token)
      setCategories((prev) => prev.map((c) => c.id === catId ? { ...c, name: updated.name } : c))
      setRenaming(null)
    } catch {
      toast.error("Couldn't rename category.")
    }
  }

  async function handleMoveCategory(catId: number, direction: "up" | "down") {
    if (!branchId || !token) return
    const cat = categories.find((c) => c.id === catId)
    if (!cat) return
    setCatMore(null)
    const newPos = direction === "up" ? cat.position - 1 : cat.position + 1
    try {
      await staffApi.updateCategory(catId, branchId, {
        name: cat.name,
        position: newPos,
        is_active: cat.is_active,
      }, token)
      await loadMenu()
    } catch {
      toast.error("Couldn't reorder category.")
    }
  }

  function openCreateModal(categoryId: number) {
    setModalState({ open: true, mode: "create", defaultCategoryId: categoryId })
  }

  function openEditModal(item: MenuItem) {
    setModalState({ open: true, mode: "edit", item })
  }

  function handleModalSaved(savedItem: MenuItem) {
    const prevCatId = modalState.item?.category_id
    if (modalState.mode === "create") {
      setCategories((prev) =>
        prev.map((cat) =>
          cat.id === savedItem.category_id ? { ...cat, items: [...cat.items, savedItem] } : cat
        )
      )
    } else if (savedItem.category_id !== prevCatId) {
      loadMenu()
    } else {
      updateItemInState(savedItem.id, savedItem)
    }
    setModalState({ open: false, mode: "create" })
  }

  function handleModalDeleted(itemId: number) {
    setCategories((prev) =>
      prev.map((cat) => ({ ...cat, items: cat.items.filter((i) => i.id !== itemId) }))
    )
    setModalState({ open: false, mode: "create" })
  }

  async function handleCreateCategory() {
    if (!branchId || !token || !newCatName.trim()) return
    setCreatingCat(true)
    try {
      const nextPos = categories.length > 0 ? Math.max(...categories.map((c) => c.position)) + 1 : 0
      const created = await staffApi.createCategory(branchId, newCatName.trim(), nextPos, token)
      setCategories((prev) => [...prev, { ...created, items: [] }])
      setExpanded((prev) => new Set([...prev, created.id]))
      setNewCatName("")
      setAddingCat(false)
    } catch {
      toast.error("Couldn't create category.")
    } finally {
      setCreatingCat(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  const btnIcon: React.CSSProperties = {
    minHeight: 32, minWidth: 32,
    display: "flex", alignItems: "center", justifyContent: "center",
    borderRadius: "var(--rad-md)",
    border: "1px solid var(--line-2)",
    background: "var(--bg-elev-2)",
    color: "var(--ink-3)",
    cursor: "pointer",
  }

  return (
    <>
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      {categories.map((cat) => {
        const isExpanded = expanded.has(cat.id)
        const isRenaming = renaming === cat.id
        const isBulking = bulkToggling === cat.id
        const showMore = catMore === cat.id

        return (
          <div key={cat.id} style={{
            borderRadius: "var(--rad-lg)",
            border: "1px solid var(--line-1)",
            background: "var(--bg-elev-1)",
            overflow: "hidden",
          }}>
            {/* Category header */}
            <div style={{
              display: "flex", alignItems: "center", gap: 8,
              padding: "12px 14px",
              borderBottom: isExpanded ? "1px solid var(--line-1)" : "none",
            }}>
              <button
                className="press"
                onClick={() => setExpanded((prev) => {
                  const next = new Set(prev)
                  if (next.has(cat.id)) next.delete(cat.id)
                  else next.add(cat.id)
                  return next
                })}
                style={{ ...btnIcon, border: "none", background: "transparent", flexShrink: 0 }}
                aria-label={isExpanded ? "Collapse" : "Expand"}
              >
                {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </button>

              {isRenaming ? (
                <input
                  autoFocus
                  value={renameValue}
                  onChange={(e) => setRenameValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleRenameCategory(cat.id)
                    if (e.key === "Escape") setRenaming(null)
                  }}
                  onBlur={() => handleRenameCategory(cat.id)}
                  style={{
                    flex: 1, fontSize: 13, fontWeight: 600,
                    background: "var(--bg-elev-2)",
                    border: "1px solid var(--accent)",
                    borderRadius: "var(--rad-sm)",
                    padding: "4px 8px", color: "var(--ink-1)",
                  }}
                />
              ) : (
                <p style={{ flex: 1, fontSize: 13, fontWeight: 600, color: "var(--ink-1)" }}>
                  {cat.name}
                  <span style={{ fontWeight: 400, color: "var(--ink-3)", marginLeft: 6 }}>
                    ({cat.items.length})
                  </span>
                  {!cat.is_active && (
                    <span style={{ fontSize: 10, fontWeight: 600, color: "var(--warn)", marginLeft: 8, textTransform: "uppercase" }}>
                      Inactive
                    </span>
                  )}
                </p>
              )}

              {isBulking && <Loader2 size={14} className="animate-spin" style={{ color: "var(--ink-3)", flexShrink: 0 }} />}

              {canManage && (
                <div style={{ display: "flex", gap: 4, flexShrink: 0 }}>
                  <button
                    className="press"
                    onClick={() => { openCreateModal(cat.id); setExpanded((prev) => new Set([...prev, cat.id])) }}
                    style={{ ...btnIcon, padding: "0 10px", gap: 4, fontSize: 12 }}
                    aria-label="Add item"
                  >
                    <Plus size={12} /> Add
                  </button>
                  <div style={{ position: "relative" }}>
                    <button
                      className="press"
                      onClick={() => setCatMore(showMore ? null : cat.id)}
                      style={btnIcon}
                      aria-label="Category options"
                    >
                      <MoreVertical size={14} />
                    </button>
                    {showMore && (
                      <div style={{
                        position: "absolute", right: 0, top: "calc(100% + 4px)", zIndex: 50,
                        minWidth: 180, background: "var(--bg-elev-3)",
                        border: "1px solid var(--line-2)", borderRadius: "var(--rad-md)",
                        boxShadow: "var(--shadow-2)", padding: "4px 0",
                      }}>
                        {[
                          { label: "Rename", action: () => { setRenaming(cat.id); setRenameValue(cat.name); setCatMore(null) } },
                          { label: "Move up", action: () => handleMoveCategory(cat.id, "up") },
                          { label: "Move down", action: () => handleMoveCategory(cat.id, "down") },
                          { label: "Mark all available", action: () => handleBulkToggle(cat.id, true) },
                          { label: "Mark all unavailable", action: () => handleBulkToggle(cat.id, false) },
                          { label: "Delete category", action: () => handleDeleteCategory(cat.id), danger: true },
                        ].map(({ label, action, danger }) => (
                          <button
                            key={label}
                            className="press"
                            onClick={action}
                            style={{
                              width: "100%", textAlign: "left",
                              padding: "8px 14px", fontSize: 13,
                              background: "transparent", border: "none", cursor: "pointer",
                              color: danger ? "var(--err)" : "var(--ink-2)",
                            }}
                          >
                            {label}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* Item rows */}
            {isExpanded && (
              <div style={{ display: "flex", flexDirection: "column" }}>
                {cat.items.map((item, idx) => (
                  <div
                    key={item.id}
                    style={{
                      display: "flex", alignItems: "center", gap: 10,
                      padding: "10px 14px",
                      borderBottom: idx < cat.items.length - 1 ? "1px solid var(--line-1)" : "none",
                      opacity: item.is_available ? 1 : 0.6,
                    }}
                  >
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {item.name}
                      </p>
                      <p style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 1 }}>
                        {formatCurrency(item.price)}
                      </p>
                    </div>
                    <div style={{ display: "flex", gap: 6, flexShrink: 0, alignItems: "center" }}>
                      {/* Availability toggle — all roles */}
                      <button
                        onClick={() => handleToggle(item.id, item.is_available)}
                        disabled={toggling === item.id}
                        className="press"
                        style={{
                          fontSize: 11, fontWeight: 600, padding: "4px 12px",
                          borderRadius: "var(--rad-pill)", border: "none",
                          minHeight: 30, display: "flex", alignItems: "center", justifyContent: "center",
                          background: item.is_available ? "var(--ok)" : "var(--bg-elev-3)",
                          color: item.is_available ? "white" : "var(--ink-3)",
                          cursor: toggling === item.id ? "not-allowed" : "pointer",
                        }}
                        aria-label={item.is_available ? "Mark unavailable" : "Mark available"}
                      >
                        {toggling === item.id ? <Loader2 size={11} className="animate-spin" /> : item.is_available ? "On" : "Off"}
                      </button>
                      {canManage && (
                        <>
                          <button className="press" onClick={() => openEditModal(item)} style={btnIcon} aria-label="Edit item">
                            <Edit2 size={12} />
                          </button>
                          <button className="press" onClick={() => handleDeleteItem(item.id, cat.id)} style={{ ...btnIcon, color: "var(--err)" }} aria-label="Delete item">
                            <Trash2 size={12} />
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                ))}

                {cat.items.length === 0 && (
                  <p style={{ padding: "16px 14px", fontSize: 13, color: "var(--ink-3)", fontStyle: "italic" }}>
                    No items yet.
                  </p>
                )}
              </div>
            )}
          </div>
        )
      })}

      {/* Add category */}
      {canManage && (
        <div>
          {addingCat ? (
            <div style={{ display: "flex", gap: 8 }}>
              <Input
                autoFocus
                placeholder="Category name *"
                value={newCatName}
                onChange={(e) => setNewCatName(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") handleCreateCategory(); if (e.key === "Escape") setAddingCat(false) }}
                style={{ flex: 1, fontSize: 13 }}
              />
              <Button onClick={handleCreateCategory} disabled={creatingCat}>
                {creatingCat ? <Loader2 size={14} className="animate-spin" /> : "Create"}
              </Button>
              <Button variant="outline" onClick={() => setAddingCat(false)}>Cancel</Button>
            </div>
          ) : (
            <button
              className="press"
              onClick={() => setAddingCat(true)}
              style={{
                width: "100%", padding: "12px", borderRadius: "var(--rad-lg)",
                border: "2px dashed var(--line-2)", background: "transparent",
                fontSize: 13, color: "var(--ink-3)", cursor: "pointer",
                display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
              }}
            >
              <Plus size={14} /> Add Category
            </button>
          )}
        </div>
      )}
    </div>

    {/* Close dropdown on outside click */}
    {catMore !== null && (
      <div
        style={{ position: "fixed", inset: 0, zIndex: 40 }}
        onClick={() => setCatMore(null)}
        aria-hidden
      />
    )}

    {/* Item modal (create & edit) */}
    {modalState.open && branchId && token && (
      <MenuItemModal
        mode={modalState.mode}
        item={modalState.item}
        categories={categories}
        defaultCategoryId={modalState.defaultCategoryId}
        branchId={branchId}
        token={token}
        onClose={() => setModalState({ open: false, mode: "create" })}
        onSaved={handleModalSaved}
        onDeleted={handleModalDeleted}
      />
    )}
    </>
  )
}
