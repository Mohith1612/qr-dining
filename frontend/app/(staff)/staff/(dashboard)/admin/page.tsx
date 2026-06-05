"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { menuApi } from "@/lib/api/menu"
import { analyticsApi, type AnalyticsPeriod, type TopItem, type BusyHour, type OrderVolumeDay } from "@/lib/api/analytics"
import { plansApi, type Subscription, type Plan } from "@/lib/api/plans"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { TopItemsList } from "@/components/analytics/TopItemsList"
import { BusyHoursChart } from "@/components/analytics/BusyHoursChart"
import { OrderVolumeChart } from "@/components/analytics/OrderVolumeChart"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode, Settings2, ChevronDown, ChevronRight, Plus, Trash2, Edit2, ChevronUp } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, MenuItem, ItemModifier, StaffRole, Table, DietaryFlag, ItemBadge } from "@/types/api"
import { tablesApi } from "@/lib/api/tables"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { StatsSkeleton, TablesSkeleton } from "@/components/shared/LoadingSkeleton"

// ─── Sessions Tab ───────────────────────────────────────────────────────────

function SessionsTab() {
  const { branchId, token } = useStaffStore()
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  const fetchSessions = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await staffApi.getActiveSessions(branchId, token)
      setSessions(data)
    } catch {
      toast.error("Couldn't load sessions.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
          {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={fetchSessions}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6,
            fontSize: 12, color: "var(--ink-3)",
            padding: "6px 10px", borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
          }}
          aria-label="Refresh sessions"
        >
          <RefreshCw size={12} aria-hidden />
          Refresh
        </button>
      </div>

      {sessions.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No active sessions"
          description="Active sessions will appear here."
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {sessions.map((s) => (
            <HospitalityCard
              key={s.id}
              elev={1}
              style={{ padding: "14px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}
            >
              <div style={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 3 }}>
                <p className="mono" style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-1)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  #{s.id.slice(0, 8)}
                </p>
                <p style={{ fontSize: 12, color: "var(--ink-3)" }}>
                  {s.table_identifier ?? `Table ${s.table_id}`} · {relativeTime(s.created_at)}
                </p>
              </div>
              <span
                style={{
                  fontSize: 11, fontWeight: 500,
                  padding: "3px 10px", borderRadius: "var(--rad-pill)",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-3)", flexShrink: 0,
                  letterSpacing: "0.03em", textTransform: "uppercase",
                }}
              >
                {s.status}
              </span>
            </HospitalityCard>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Menu Tab ────────────────────────────────────────────────────────────────

const DIETARY_OPTIONS: { flag: DietaryFlag; label: string }[] = [
  { flag: "vegetarian", label: "Vegetarian" },
  { flag: "vegan", label: "Vegan" },
  { flag: "jain", label: "Jain" },
  { flag: "egg", label: "Contains Egg" },
  { flag: "non-veg", label: "Non-Vegetarian" },
]

const BADGE_OPTIONS: { badge: ItemBadge; label: string }[] = [
  { badge: "chef-special", label: "Chef Special" },
  { badge: "bestseller", label: "Bestseller" },
  { badge: "seasonal", label: "Seasonal" },
  { badge: "new", label: "New" },
]

const SPICE_LABELS = ["None", "Mild", "Medium", "Hot"]

function applyDietaryLogic(flags: DietaryFlag[], toggled: DietaryFlag, checked: boolean): DietaryFlag[] {
  let next: DietaryFlag[] = checked ? [...flags, toggled] : flags.filter((f) => f !== toggled)
  if (checked) {
    if (toggled === "jain") next = [...new Set([...next, "vegan" as DietaryFlag, "vegetarian" as DietaryFlag])]
    if (toggled === "vegan") next = [...new Set([...next, "vegetarian" as DietaryFlag])]
    if (toggled === "non-veg") next = next.filter((f) => !(["vegetarian", "vegan", "jain", "egg"] as DietaryFlag[]).includes(f))
    if ((["vegetarian", "vegan", "jain"] as DietaryFlag[]).includes(toggled)) next = next.filter((f) => f !== "non-veg")
  }
  return next
}

interface EditItemForm {
  name: string
  description: string
  price: string
  position: number
  categoryId: number
  dietaryFlags: DietaryFlag[]
  itemBadges: ItemBadge[]
  spiceLevel: number
  isFeatured: boolean
  featuredSortOrder: number
  saving: boolean
  errors: { name?: string; price?: string }
}

interface NewModForm {
  name: string
  modifier_group: string
  price_delta: string
  is_required: boolean
}

function MenuTab() {
  const { branchId, token, role } = useStaffStore()
  const canManage = role === "owner" || role === "manager"

  const [categories, setCategories] = useState<MenuCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  // Per-item states
  const [toggling, setToggling] = useState<number | null>(null)
  const [bulkToggling, setBulkToggling] = useState<number | null>(null) // category ID

  // Edit item sheet
  const [editItem, setEditItem] = useState<MenuItem | null>(null)
  const [editForm, setEditForm] = useState<EditItemForm | null>(null)
  const [addingMod, setAddingMod] = useState(false)
  const [newMod, setNewMod] = useState<NewModForm>({ name: "", modifier_group: "", price_delta: "0", is_required: false })
  const [savingMod, setSavingMod] = useState(false)

  // Category management
  const [catMore, setCatMore] = useState<number | null>(null) // category ID with open dropdown
  const [renaming, setRenaming] = useState<number | null>(null)
  const [renameValue, setRenameValue] = useState("")

  // Add item inline form
  const [addItemCat, setAddItemCat] = useState<number | null>(null)
  const [newItemForm, setNewItemForm] = useState({ name: "", price: "", description: "" })
  const [addingItem, setAddingItem] = useState(false)

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

  function openEditSheet(item: MenuItem) {
    setEditItem(item)
    setEditForm({
      name: item.name,
      description: item.description,
      price: item.price,
      position: item.position,
      categoryId: item.category_id,
      dietaryFlags: item.dietary_flags ?? [],
      itemBadges: item.item_badges ?? [],
      spiceLevel: item.spice_level ?? 0,
      isFeatured: item.is_featured ?? false,
      featuredSortOrder: item.featured_sort_order ?? 0,
      saving: false,
      errors: {},
    })
    setAddingMod(false)
    setNewMod({ name: "", modifier_group: "", price_delta: "0", is_required: false })
  }

  async function handleSaveItem() {
    if (!editForm || !editItem || !branchId || !token) return
    const errors: EditItemForm["errors"] = {}
    if (!editForm.name.trim()) errors.name = "Name is required"
    const priceNum = parseFloat(editForm.price)
    if (isNaN(priceNum) || priceNum < 0) errors.price = "Price must be ≥ 0"
    if (Object.keys(errors).length) {
      setEditForm((f) => f && { ...f, errors })
      return
    }
    setEditForm((f) => f && { ...f, saving: true, errors: {} })
    try {
      const updated = await staffApi.updateMenuItem(
        editItem.id,
        branchId,
        {
          name: editForm.name.trim(),
          price: priceNum,
          description: editForm.description,
          position: editForm.position,
          dietary_flags: editForm.dietaryFlags,
          item_badges: editForm.itemBadges,
          spice_level: editForm.spiceLevel,
          category_id: editForm.categoryId !== editItem.category_id ? editForm.categoryId : undefined,
        },
        token
      )
      // Also update featured separately if changed
      if (editForm.isFeatured !== (editItem.is_featured ?? false) ||
          editForm.featuredSortOrder !== (editItem.featured_sort_order ?? 0)) {
        await staffApi.toggleFeatured(editItem.id, branchId, editForm.isFeatured, editForm.featuredSortOrder, token)
      }
      updateItemInState(editItem.id, { ...updated, is_featured: editForm.isFeatured, featured_sort_order: editForm.featuredSortOrder })
      // If category changed, reload to move the item
      if (editForm.categoryId !== editItem.category_id) {
        await loadMenu()
      }
      toast.success("Item saved")
      setEditItem(null)
      setEditForm(null)
    } catch {
      toast.error("Couldn't save item.")
      setEditForm((f) => f && { ...f, saving: false })
    }
  }

  async function handleAddModifier() {
    if (!editItem || !branchId || !token) return
    if (!newMod.name.trim()) { toast.error("Modifier name required"); return }
    setSavingMod(true)
    try {
      const created = await staffApi.addModifier(editItem.id, branchId, {
        name: newMod.name.trim(),
        price_delta: parseFloat(newMod.price_delta) || 0,
        is_required: newMod.is_required,
        modifier_group: newMod.modifier_group.trim(),
      }, token)
      updateItemInState(editItem.id, {
        modifiers: [...(editItem.modifiers ?? []), created],
      })
      setEditItem((prev) => prev && { ...prev, modifiers: [...(prev.modifiers ?? []), created] })
      setNewMod({ name: "", modifier_group: "", price_delta: "0", is_required: false })
      setAddingMod(false)
    } catch {
      toast.error("Couldn't add modifier.")
    } finally {
      setSavingMod(false)
    }
  }

  async function handleDeleteModifier(modId: number) {
    if (!editItem || !branchId || !token) return
    try {
      await staffApi.deleteModifier(modId, branchId, token)
      const updatedMods = (editItem.modifiers ?? []).filter((m) => m.id !== modId)
      updateItemInState(editItem.id, { modifiers: updatedMods })
      setEditItem((prev) => prev && { ...prev, modifiers: updatedMods })
    } catch {
      toast.error("Couldn't delete modifier.")
    }
  }

  async function handleCreateItem(catId: number) {
    if (!branchId || !token) return
    if (!newItemForm.name.trim() || !newItemForm.price.trim()) {
      toast.error("Name and price are required")
      return
    }
    const price = parseFloat(newItemForm.price)
    if (isNaN(price) || price < 0) { toast.error("Invalid price"); return }
    setAddingItem(true)
    try {
      const created = await staffApi.createMenuItem(branchId, catId, newItemForm.name.trim(), price, token, {
        description: newItemForm.description,
        is_available: true,
      })
      setCategories((prev) =>
        prev.map((cat) => cat.id === catId ? { ...cat, items: [...cat.items, created] } : cat)
      )
      setNewItemForm({ name: "", price: "", description: "" })
      setAddItemCat(null)
    } catch {
      toast.error("Couldn't create item.")
    } finally {
      setAddingItem(false)
    }
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
                    onClick={() => { setAddItemCat(addItemCat === cat.id ? null : cat.id); setExpanded((prev) => new Set([...prev, cat.id])) }}
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
                          <button className="press" onClick={() => openEditSheet(item)} style={btnIcon} aria-label="Edit item">
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

                {/* Inline add item form */}
                {addItemCat === cat.id && canManage && (
                  <div style={{ padding: "12px 14px", borderTop: "1px solid var(--line-1)", display: "flex", flexDirection: "column", gap: 8 }}>
                    <Input
                      placeholder="Item name *"
                      value={newItemForm.name}
                      onChange={(e) => setNewItemForm((f) => ({ ...f, name: e.target.value }))}
                      style={{ fontSize: 13 }}
                    />
                    <div style={{ display: "flex", gap: 8 }}>
                      <Input
                        placeholder="Price *"
                        type="number"
                        min={0}
                        step={0.5}
                        value={newItemForm.price}
                        onChange={(e) => setNewItemForm((f) => ({ ...f, price: e.target.value }))}
                        style={{ flex: 1, fontSize: 13 }}
                      />
                      <Input
                        placeholder="Description"
                        value={newItemForm.description}
                        onChange={(e) => setNewItemForm((f) => ({ ...f, description: e.target.value }))}
                        style={{ flex: 2, fontSize: 13 }}
                      />
                    </div>
                    <div style={{ display: "flex", gap: 8 }}>
                      <Button
                        onClick={() => handleCreateItem(cat.id)}
                        disabled={addingItem}
                        style={{ flex: 1 }}
                      >
                        {addingItem ? <Loader2 size={14} className="animate-spin" /> : "Add Item"}
                      </Button>
                      <Button variant="outline" onClick={() => setAddItemCat(null)} style={{ flex: 1 }}>
                        Cancel
                      </Button>
                    </div>
                  </div>
                )}

                {cat.items.length === 0 && addItemCat !== cat.id && (
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

    {/* Edit item sheet */}
    {editItem && editForm && (
      <BottomSheet
        open={true}
        onClose={() => { setEditItem(null); setEditForm(null) }}
        title={`Edit: ${editItem.name}`}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 20, paddingBottom: 20 }}>
          {/* Name */}
          <div>
            <label style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-3)", display: "block", marginBottom: 6 }}>Name *</label>
            <Input
              value={editForm.name}
              maxLength={80}
              onChange={(e) => setEditForm((f) => f && { ...f, name: e.target.value, errors: { ...f.errors, name: undefined } })}
              style={{ borderColor: editForm.errors.name ? "var(--err)" : undefined }}
            />
            {editForm.errors.name && <p style={{ fontSize: 11, color: "var(--err)", marginTop: 4 }}>{editForm.errors.name}</p>}
          </div>

          {/* Description */}
          <div>
            <label style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-3)", display: "block", marginBottom: 6 }}>Description</label>
            <textarea
              value={editForm.description}
              rows={2}
              onChange={(e) => setEditForm((f) => f && { ...f, description: e.target.value })}
              style={{
                width: "100%", fontSize: 14, padding: "8px 12px",
                borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
                background: "var(--bg-elev-2)", color: "var(--ink-1)", resize: "none",
              }}
            />
          </div>

          {/* Price and Position */}
          <div style={{ display: "flex", gap: 12 }}>
            <div style={{ flex: 2 }}>
              <label style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-3)", display: "block", marginBottom: 6 }}>Price (₹) *</label>
              <Input
                type="number"
                min={0}
                step={0.5}
                value={editForm.price}
                onChange={(e) => setEditForm((f) => f && { ...f, price: e.target.value, errors: { ...f.errors, price: undefined } })}
                style={{ borderColor: editForm.errors.price ? "var(--err)" : undefined }}
              />
              {editForm.errors.price && <p style={{ fontSize: 11, color: "var(--err)", marginTop: 4 }}>{editForm.errors.price}</p>}
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-3)", display: "block", marginBottom: 6 }}>Position</label>
              <Input
                type="number"
                min={0}
                value={editForm.position}
                onChange={(e) => setEditForm((f) => f && { ...f, position: parseInt(e.target.value) || 0 })}
              />
            </div>
          </div>

          {/* Category */}
          <div>
            <label style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-3)", display: "block", marginBottom: 6 }}>Category</label>
            <select
              value={editForm.categoryId}
              onChange={(e) => setEditForm((f) => f && { ...f, categoryId: parseInt(e.target.value) })}
              style={{
                width: "100%", fontSize: 14, padding: "8px 12px",
                borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
                background: "var(--bg-elev-2)", color: "var(--ink-1)",
              }}
            >
              {categories.map((cat) => (
                <option key={cat.id} value={cat.id}>{cat.name}</option>
              ))}
            </select>
          </div>

          {/* Dietary flags */}
          <div>
            <p className="eyebrow" style={{ marginBottom: 10 }}>Dietary</p>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {DIETARY_OPTIONS.map(({ flag, label }) => (
                <label key={flag} style={{ display: "flex", alignItems: "center", gap: 10, cursor: "pointer" }}>
                  <input
                    type="checkbox"
                    checked={editForm.dietaryFlags.includes(flag)}
                    onChange={(e) => setEditForm((f) => f && {
                      ...f,
                      dietaryFlags: applyDietaryLogic(f.dietaryFlags, flag, e.target.checked),
                    })}
                    style={{ width: 18, height: 18, accentColor: "var(--accent)", cursor: "pointer" }}
                  />
                  <span style={{ fontSize: 14, color: "var(--ink-2)" }}>{label}</span>
                </label>
              ))}
            </div>
          </div>

          {/* Badges */}
          <div>
            <p className="eyebrow" style={{ marginBottom: 10 }}>Badges</p>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
              {BADGE_OPTIONS.map(({ badge, label }) => {
                const active = editForm.itemBadges.includes(badge)
                return (
                  <button
                    key={badge}
                    className="press"
                    onClick={() => setEditForm((f) => f && {
                      ...f,
                      itemBadges: active ? f.itemBadges.filter((b) => b !== badge) : [...f.itemBadges, badge],
                    })}
                    style={{
                      fontSize: 13, fontWeight: 600, padding: "8px 16px",
                      borderRadius: "var(--rad-pill)", border: "1px solid",
                      borderColor: active ? "var(--accent)" : "var(--line-2)",
                      background: active ? "var(--accent-soft)" : "var(--bg-elev-2)",
                      color: active ? "var(--accent)" : "var(--ink-3)",
                      cursor: "pointer",
                    }}
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Spice */}
          <div>
            <p className="eyebrow" style={{ marginBottom: 10 }}>Spice Level</p>
            <div style={{ display: "flex", gap: 8 }}>
              {SPICE_LABELS.map((label, level) => {
                const active = editForm.spiceLevel === level
                return (
                  <button
                    key={level}
                    className="press"
                    onClick={() => setEditForm((f) => f && { ...f, spiceLevel: level })}
                    style={{
                      flex: 1, fontSize: 12, fontWeight: 600, padding: "8px 4px",
                      borderRadius: "var(--rad-md)", border: "1px solid",
                      borderColor: active ? "var(--accent)" : "var(--line-2)",
                      background: active ? "var(--accent-soft)" : "var(--bg-elev-2)",
                      color: active ? "var(--accent)" : "var(--ink-3)",
                      cursor: "pointer",
                    }}
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Featured */}
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <label style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={editForm.isFeatured}
                onChange={(e) => setEditForm((f) => f && { ...f, isFeatured: e.target.checked })}
                style={{ width: 18, height: 18, accentColor: "var(--accent)" }}
              />
              <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-2)" }}>Feature this item</span>
            </label>
            {editForm.isFeatured && (
              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <span style={{ fontSize: 12, color: "var(--ink-3)" }}>Sort</span>
                <Input
                  type="number"
                  min={0}
                  value={editForm.featuredSortOrder}
                  onChange={(e) => setEditForm((f) => f && { ...f, featuredSortOrder: parseInt(e.target.value) || 0 })}
                  style={{ width: 70 }}
                />
              </div>
            )}
          </div>

          {/* Modifiers */}
          <div>
            <p className="eyebrow" style={{ marginBottom: 10 }}>Modifiers</p>
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              {(editItem.modifiers ?? []).map((mod) => (
                <div
                  key={mod.id}
                  style={{
                    display: "flex", alignItems: "center", gap: 8,
                    padding: "8px 12px",
                    borderRadius: "var(--rad-md)",
                    background: "var(--bg-elev-2)",
                    border: "1px solid var(--line-1)",
                  }}
                >
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)" }}>{mod.name}</p>
                    <p style={{ fontSize: 11, color: "var(--ink-3)" }}>
                      {mod.modifier_group && <span style={{ marginRight: 8 }}>{mod.modifier_group}</span>}
                      {mod.price_delta >= 0 ? `+₹${mod.price_delta}` : `-₹${Math.abs(mod.price_delta)}`}
                      {mod.is_required && <span style={{ marginLeft: 8, color: "var(--accent)" }}>Required</span>}
                    </p>
                  </div>
                  <button
                    className="press"
                    onClick={() => handleDeleteModifier(mod.id)}
                    style={{ ...btnIcon, color: "var(--err)" }}
                    aria-label="Delete modifier"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              ))}

              {addingMod ? (
                <div style={{ display: "flex", flexDirection: "column", gap: 8, padding: "10px 12px", background: "var(--bg-elev-2)", borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)" }}>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
                    <Input
                      placeholder="Name (e.g. Large)"
                      value={newMod.name}
                      onChange={(e) => setNewMod((m) => ({ ...m, name: e.target.value }))}
                      style={{ fontSize: 13 }}
                    />
                    <Input
                      placeholder="Group (e.g. size)"
                      value={newMod.modifier_group}
                      onChange={(e) => setNewMod((m) => ({ ...m, modifier_group: e.target.value }))}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    <Input
                      type="number"
                      placeholder="₹ delta"
                      value={newMod.price_delta}
                      onChange={(e) => setNewMod((m) => ({ ...m, price_delta: e.target.value }))}
                      style={{ flex: 1, fontSize: 13 }}
                    />
                    <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--ink-2)", cursor: "pointer", flexShrink: 0 }}>
                      <input
                        type="checkbox"
                        checked={newMod.is_required}
                        onChange={(e) => setNewMod((m) => ({ ...m, is_required: e.target.checked }))}
                        style={{ accentColor: "var(--accent)" }}
                      />
                      Required
                    </label>
                  </div>
                  <div style={{ display: "flex", gap: 8 }}>
                    <Button onClick={handleAddModifier} disabled={savingMod} style={{ flex: 1 }}>
                      {savingMod ? <Loader2 size={14} className="animate-spin" /> : "Add"}
                    </Button>
                    <Button variant="outline" onClick={() => setAddingMod(false)} style={{ flex: 1 }}>Cancel</Button>
                  </div>
                </div>
              ) : (
                <button
                  className="press"
                  onClick={() => setAddingMod(true)}
                  style={{
                    padding: "8px 12px", borderRadius: "var(--rad-md)",
                    border: "1px dashed var(--line-2)", background: "transparent",
                    fontSize: 13, color: "var(--ink-3)", cursor: "pointer",
                    display: "flex", alignItems: "center", gap: 6,
                  }}
                >
                  <Plus size={12} /> Add modifier
                </button>
              )}
            </div>
          </div>

          {/* Save */}
          <button
            onClick={handleSaveItem}
            disabled={editForm.saving}
            className="press btn-primary"
            style={{
              width: "100%", height: 52, borderRadius: 14,
              display: "flex", alignItems: "center", justifyContent: "center",
              fontSize: 15, fontWeight: 700,
              opacity: editForm.saving ? 0.6 : 1,
              cursor: editForm.saving ? "not-allowed" : "pointer",
            }}
          >
            {editForm.saving ? <Loader2 size={16} className="animate-spin" /> : "Save Changes"}
          </button>
        </div>
      </BottomSheet>
    )}
    </>
  )
}

// ─── Staff Tab ───────────────────────────────────────────────────────────────

const STAFF_ROLES: { value: StaffRole; label: string }[] = [
  { value: "waiter",  label: "Waiter"  },
  { value: "kitchen", label: "Kitchen" },
  { value: "manager", label: "Manager" },
]

function StaffTab() {
  const { branchId, token, role } = useStaffStore()
  const [name, setName] = useState("")
  const [selectedRole, setSelectedRole] = useState<StaffRole>("waiter")
  const [pin, setPin] = useState("")
  const [creating, setCreating] = useState(false)

  const canManage = role === "owner" || role === "manager"

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!branchId || !token || !name || !pin) return
    setCreating(true)
    try {
      await staffApi.createStaff(branchId, name, selectedRole, pin, token)
      toast.success(`${name} added as ${selectedRole}`)
      setName("")
      setPin("")
    } catch {
      toast.error("Couldn't create staff account.")
    } finally {
      setCreating(false)
    }
  }

  if (!canManage) {
    return (
      <div style={{ padding: "48px 0", textAlign: "center", color: "var(--ink-3)" }}>
        <p style={{ fontSize: 14 }}>Only owners and managers can manage staff accounts.</p>
      </div>
    )
  }

  return (
    <div>
      <HospitalityCard elev={2} style={{ padding: 20 }}>
        <p className="serif" style={{ fontSize: 18, fontWeight: 500, color: "var(--ink-1)", marginBottom: 16 }}>
          Add staff member
        </p>

        <form onSubmit={handleCreate} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Staff name"
              required
              style={{
                height: 44,
                borderRadius: "var(--rad-lg)",
                background: "var(--bg-base)",
                borderColor: "var(--line-2)",
                color: "var(--ink-1)",
              }}
            />
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              Role
            </label>
            <div style={{ display: "flex", gap: 6 }}>
              {STAFF_ROLES.map(({ value, label }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setSelectedRole(value)}
                  className="press"
                  style={{
                    flex: 1, padding: "10px 8px",
                    borderRadius: "var(--rad-md)",
                    fontSize: 13, fontWeight: 500,
                    border: "1px solid",
                    borderColor: selectedRole === value ? "var(--accent)" : "var(--line-2)",
                    background: selectedRole === value ? "var(--accent-soft)" : "var(--bg-base)",
                    color: selectedRole === value ? "var(--accent)" : "var(--ink-3)",
                    minHeight: 44,
                    transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
                  }}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              PIN
            </label>
            <Input
              type="password"
              inputMode="numeric"
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              placeholder="4–6 digits"
              maxLength={6}
              required
              style={{
                height: 44,
                borderRadius: "var(--rad-lg)",
                background: "var(--bg-base)",
                borderColor: "var(--line-2)",
                color: "var(--ink-1)",
                letterSpacing: "0.2em",
              }}
            />
          </div>

          <Button
            type="submit"
            disabled={creating || !name || !pin}
            style={{
              width: "100%", height: 44,
              borderRadius: "var(--rad-lg)",
              background: creating ? "var(--bg-elev-3)" : "linear-gradient(180deg, var(--accent-strong), var(--accent))",
              color: creating ? "var(--ink-3)" : "var(--accent-ink)",
              border: "none", fontSize: 14, fontWeight: 600,
            }}
          >
            {creating ? (
              <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Loader2 size={16} className="animate-spin" />
                Creating…
              </span>
            ) : (
              "Create account"
            )}
          </Button>
        </form>
      </HospitalityCard>
    </div>
  )
}

// ─── Stats Tab ───────────────────────────────────────────────────────────────

function StatsTab() {
  const { branchId, token } = useStaffStore()
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [topItems, setTopItems] = useState<TopItem[]>([])
  const [busyHours, setBusyHours] = useState<BusyHour[]>([])
  const [orderVolume, setOrderVolume] = useState<OrderVolumeDay[]>([])
  const [gated, setGated] = useState(false)
  const [loading, setLoading] = useState(true)

  const fetchAnalytics = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    setGated(false)
    try {
      const [topResult, busyResult, volumeResult] = await Promise.all([
        analyticsApi.getTopItems(branchId, period, token),
        analyticsApi.getBusyHours(branchId, period, token),
        analyticsApi.getOrderVolume(branchId, period, token),
      ])
      setTopItems(topResult.items)
      setBusyHours(busyResult.hours)
      setOrderVolume(volumeResult.days)
    } catch (err) {
      if (err instanceof ApiError && err.code === "ANALYTICS_GATED") {
        setGated(true)
      } else {
        toast.error("Couldn't load analytics.")
      }
    } finally {
      setLoading(false)
    }
  }, [branchId, token, period])

  useEffect(() => {
    fetchAnalytics()
  }, [fetchAnalytics])

  if (loading) return <StatsSkeleton />

  if (gated) {
    return (
      <EmptyState
        icon={BarChart2}
        title="Analytics not available"
        description="Analytics is available on Standard and Premium plans."
        action={{ label: "View pricing", onClick: () => window.location.href = "/pricing" }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <PeriodSelector value={period} onChange={setPeriod} />

      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Top items</SectionHeader>
        <TopItemsList items={topItems} />
      </HospitalityCard>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <HospitalityCard elev={1} className="p-5">
          <SectionHeader className="mb-4">Busy hours</SectionHeader>
          <BusyHoursChart hours={busyHours} />
        </HospitalityCard>

        <HospitalityCard elev={1} className="p-5">
          <SectionHeader className="mb-4">Order volume</SectionHeader>
          <OrderVolumeChart days={orderVolume} />
        </HospitalityCard>
      </div>
    </div>
  )
}

// ─── Plan Tab ────────────────────────────────────────────────────────────────

function formatFeatureVal(key: keyof Plan["features_json"], val: number | boolean): string {
  if (typeof val === "boolean") return val ? "✓" : "—"
  return val === -1 ? "Unlimited" : String(val)
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleDateString("en", { month: "short", day: "numeric", year: "numeric" })
  } catch {
    return dateStr
  }
}

const PLAN_FEATURES: { label: string; key: keyof Plan["features_json"] }[] = [
  { label: "Branches",     key: "max_branches"  },
  { label: "Tables",       key: "max_tables"     },
  { label: "Analytics",    key: "analytics"      },
  { label: "Multi-branch", key: "multi_branch"   },
]

function PlanTab() {
  const { token } = useStaffStore()
  const { restaurantId, ready: tenantReady } = useTenant()
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [features, setFeatures] = useState<Plan["features_json"] | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!tenantReady) return
    if (!restaurantId || !token) {
      setLoading(false)
      return
    }
    plansApi
      .getSubscription(restaurantId, token)
      .then((data) => {
        setSubscription(data.subscription)
        setFeatures(data.features)
      })
      .catch(() => toast.error("Couldn't load subscription."))
      .finally(() => setLoading(false))
  }, [restaurantId, token, tenantReady])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin opacity-40" />
      </div>
    )
  }

  if (!restaurantId) {
    return (
      <div className="py-12 text-center">
        <p className="text-[13px] text-[var(--ink-3)]">
          No tenant context. Set{" "}
          <code className="font-mono text-[11px] px-1.5 py-0.5 rounded-[var(--rad-sm)] bg-[var(--bg-elev-2)] border border-[var(--line-2)] text-[var(--ink-2)]">
            NEXT_PUBLIC_TENANT_SLUG
          </code>{" "}
          in your local environment.
        </p>
      </div>
    )
  }

  if (!subscription || !features) {
    return <EmptyState icon={CreditCard} title="No subscription found." />
  }

  const isPaid = subscription.plan_tier !== "free"
  const isActive = subscription.status === "active"
  const isTrial = subscription.status === "trial"

  return (
    <div>
      <HospitalityCard elev={2} className="p-5">
        <div className="flex items-center gap-2 flex-wrap mb-1">
          <span className="serif text-lg font-medium text-[var(--ink-1)]">
            {subscription.plan_name}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest border",
              isPaid
                ? "bg-[var(--accent-soft)] text-[var(--accent)] border-[var(--accent)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)] border-[var(--line-2)]"
            )}
          >
            {subscription.plan_tier}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest",
              isActive
                ? "bg-[var(--ok-soft)] text-[var(--ok)]"
                : isTrial
                ? "bg-[var(--warn-soft)] text-[var(--warn)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)]"
            )}
          >
            {subscription.status}
          </span>
        </div>

        {isTrial && subscription.trial_ends_at && (
          <p className="text-[13px] text-[var(--ink-3)] mb-4">
            Trial ends {formatDate(subscription.trial_ends_at)}
          </p>
        )}

        <hr className="rule-strong my-4" />

        <div className="flex flex-col mb-5">
          {PLAN_FEATURES.map(({ label, key }, i) => {
            const val = features[key]
            const inactive = (typeof val === "boolean" && !val) || val === 0
            return (
              <div key={key}>
                {i > 0 && <hr className="rule" />}
                <div className="flex justify-between items-center py-2.5">
                  <span className="text-[13px] text-[var(--ink-3)]">{label}</span>
                  <span className={cn("serif text-[15px] font-medium", inactive ? "text-[var(--ink-4)]" : "text-[var(--ink-1)]")}>
                    {formatFeatureVal(key, val)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>

        <Link
          href="/pricing"
          className="press block text-center py-2.5 rounded-[var(--rad-md)] border border-[var(--line-2)] text-[13px] font-medium text-[var(--ink-2)] no-underline"
        >
          View all plans
        </Link>
      </HospitalityCard>
    </div>
  )
}

// ─── Tables Tab ──────────────────────────────────────────────────────────────

function TableRow({
  table,
  canManage,
  onPrint,
  onRefreshQR,
}: {
  table: Table
  canManage: boolean
  onPrint: (t: Table) => void
  onRefreshQR: (t: Table) => void
}) {
  const isOccupied = table.status === "occupied"

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "10px 14px",
        background: "var(--bg-elev-1)",
        border: "1px solid var(--line-1)",
        borderRadius: "var(--rad-md)",
      }}
    >
      {/* Identifier + capacity */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <span className="serif" style={{ fontSize: 18, color: "var(--ink-1)", fontWeight: 500 }}>
          {table.identifier}
        </span>
        <span style={{ fontSize: 12, color: "var(--ink-3)", marginLeft: 8 }}>
          {table.capacity} seats
        </span>
      </div>

      {/* Status pill */}
      <span
        style={{
          fontSize: 11,
          fontWeight: 600,
          letterSpacing: "0.06em",
          textTransform: "uppercase",
          padding: "2px 8px",
          borderRadius: 99,
          background: isOccupied ? "rgba(251,191,36,0.12)" : "rgba(52,211,153,0.12)",
          color: isOccupied ? "#F59E0B" : "#10B981",
          flexShrink: 0,
        }}
      >
        {isOccupied ? "Occupied" : "Available"}
      </span>

      {/* QR preview */}
      <QRCard table={table} size={48} />

      {/* Actions */}
      <button
        onClick={() => onPrint(table)}
        className="press"
        title="Print QR card"
        style={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          fontSize: 12,
          color: "var(--ink-2)",
          padding: "5px 10px",
          borderRadius: "var(--rad-md)",
          background: "var(--bg-elev-2)",
          border: "1px solid var(--line-1)",
          flexShrink: 0,
        }}
      >
        <Printer size={12} aria-hidden />
        Print
      </button>

      {canManage && (
        <button
          onClick={() => onRefreshQR(table)}
          disabled={isOccupied}
          className="press"
          title={isOccupied ? "Cannot regenerate while table is occupied" : "Regenerate QR token"}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 4,
            fontSize: 12,
            padding: "5px 10px",
            borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)",
            border: "1px solid var(--line-1)",
            flexShrink: 0,
            color: isOccupied ? "var(--ink-4)" : "var(--ink-2)",
            cursor: isOccupied ? "not-allowed" : "pointer",
            opacity: isOccupied ? 0.5 : 1,
          }}
        >
          <RotateCcw size={12} aria-hidden />
          Regen QR
        </button>
      )}
    </div>
  )
}

function TablesTab() {
  const { branchId, token, role } = useStaffStore()
  const { slug } = useTenant()
  const [tables, setTables] = useState<Table[]>([])
  const [loading, setLoading] = useState(true)
  const [createModal, setCreateModal] = useState(false)
  const [creating, setCreating] = useState(false)
  const [identifier, setIdentifier] = useState("")
  const [capacity, setCapacity] = useState(4)
  const [printTable, setPrintTable] = useState<Table | null>(null)

  const canManage = role === "owner" || role === "manager"

  const fetchTables = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await tablesApi.list(branchId, token)
      setTables(data)
    } catch {
      toast.error("Couldn't load tables.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchTables()
  }, [fetchTables])

  async function handleCreate() {
    if (!branchId || !token || !identifier.trim()) return
    setCreating(true)
    try {
      const table = await tablesApi.create(branchId, { identifier: identifier.trim(), capacity }, token)
      setTables(prev => [...prev, table].sort((a, b) => a.identifier.localeCompare(b.identifier)))
      setCreateModal(false)
      setIdentifier("")
      setCapacity(4)
      toast.success(`Table ${table.identifier} created.`)
    } catch {
      toast.error("Failed to create table.")
    } finally {
      setCreating(false)
    }
  }

  async function handleRefreshQR(table: Table) {
    if (!token) return
    if (!confirm(`Regenerate QR for ${table.identifier}? This will invalidate all printed cards.`)) return
    try {
      const updated = await tablesApi.refreshQR(table.id, token)
      setTables(prev => prev.map(t => t.id === updated.id ? updated : t))
      toast.success(`QR token regenerated for ${table.identifier}.`)
    } catch {
      toast.error("Failed to regenerate QR token.")
    }
  }

  function handlePrint(table: Table) {
    setPrintTable(table)
    setTimeout(() => window.print(), 100)
  }

  if (loading) return <TablesSkeleton />

  return (
    <div>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <p className="eyebrow">{tables.length} table{tables.length !== 1 ? "s" : ""}</p>
        {canManage && (
          <button
            onClick={() => setCreateModal(true)}
            className="press"
            style={{
              height: 36,
              padding: "0 16px",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 600,
              background: "var(--brass)",
              color: "#0E0C09",
              border: "none",
              cursor: "pointer",
            }}
          >
            Add table
          </button>
        )}
      </div>

      {/* Table list */}
      {tables.length === 0 ? (
        <EmptyState icon={QrCode} title="No tables yet" description="Add your first table to generate a QR code." />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {tables.map(table => (
            <TableRow
              key={table.id}
              table={table}
              canManage={canManage}
              onPrint={handlePrint}
              onRefreshQR={handleRefreshQR}
            />
          ))}
        </div>
      )}

      {/* Create modal */}
      {createModal && (
        <BottomSheet open onClose={() => setCreateModal(false)} title="Add table">
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Table identifier</p>
              <Input
                placeholder="e.g. T3, Table 12, Garden 2"
                value={identifier}
                onChange={e => setIdentifier(e.target.value)}
                onKeyDown={e => e.key === "Enter" && handleCreate()}
              />
            </div>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Capacity</p>
              <Input
                type="number"
                min={1}
                max={20}
                value={capacity}
                onChange={e => setCapacity(Number(e.target.value))}
              />
            </div>
            <button
              onClick={handleCreate}
              disabled={creating || !identifier.trim()}
              className="press"
              style={{
                height: 44,
                borderRadius: "var(--rad-md)",
                fontSize: 14,
                fontWeight: 600,
                background: "var(--brass)",
                color: "#0E0C09",
                border: "none",
                cursor: creating || !identifier.trim() ? "not-allowed" : "pointer",
                opacity: creating || !identifier.trim() ? 0.6 : 1,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                gap: 8,
              }}
            >
              {creating && <Loader2 size={14} className="animate-spin" />}
              Create table
            </button>
          </div>
        </BottomSheet>
      )}

      {/* Hidden print template — rendered when printTable is set */}
      {printTable && <PrintTemplate table={printTable} tenantSlug={slug} />}
    </div>
  )
}

// ─── Settings Tab ────────────────────────────────────────────────────────────

function SettingsTab() {
  const { branchId, token, role } = useStaffStore()
  const [timeoutMinutes, setTimeoutMinutes] = useState(120)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then((b) => setTimeoutMinutes(b.session_timeout_minutes))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [branchId, token])

  async function handleSave() {
    if (!branchId || !token) return
    setSaving(true)
    try {
      await staffApi.updateBranch(branchId, { session_timeout_minutes: timeoutMinutes }, token)
      toast.success("Settings saved")
    } catch {
      toast.error("Failed to save settings")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
      <p className="eyebrow" style={{ marginBottom: 6 }}>Session timeout</p>
      <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
        Sessions are automatically closed after this many minutes of inactivity.
      </p>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <Input
          type="number"
          min={15}
          max={480}
          step={15}
          value={timeoutMinutes}
          onChange={(e) => setTimeoutMinutes(parseInt(e.target.value) || 120)}
          disabled={!canEdit}
          style={{ width: 90 }}
        />
        <span className="text-sm" style={{ color: "var(--ink-2)" }}>minutes</span>
        {canEdit && (
          <Button
            onClick={handleSave}
            disabled={saving}
            style={{ marginLeft: 8 }}
          >
            {saving ? <Loader2 className="size-4 animate-spin" /> : "Save"}
          </Button>
        )}
      </div>
    </HospitalityCard>
  )
}

// ─── Page ────────────────────────────────────────────────────────────────────

const TABS = [
  { id: "sessions", label: "Sessions" },
  { id: "menu",     label: "Menu"     },
  { id: "tables",   label: "Tables"   },
  { id: "staff",    label: "Staff"    },
  { id: "stats",    label: "Stats"    },
  { id: "plan",     label: "Plan"     },
  { id: "settings", label: "Settings" },
]

export default function AdminPage() {
  const [activeTab, setActiveTab] = useState("sessions")

  return (
    <div className="screen-enter bg-[var(--bg-base)] text-[var(--ink-1)] min-h-screen px-5 py-6">
      {/* Heading */}
      <p className="eyebrow">Operations</p>
      <h1 className="display-xl mt-1.5">House overview</h1>

      {/* Pill tab switcher */}
      <div className="hscroll mt-6 overflow-x-auto pb-0.5">
        <div className="inline-flex gap-0.5 p-[3px] bg-[var(--bg-elev-1)] rounded-full border border-[var(--line-1)]">
          {TABS.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id)}
              className={cn(
                "press px-[18px] py-2 rounded-full border-0 text-[13px] cursor-pointer whitespace-nowrap transition-[background,color,box-shadow] duration-[var(--dur-fast)]",
                activeTab === id
                  ? "font-semibold bg-[var(--bg-elev-3)] text-[var(--ink-1)] shadow-[var(--shadow-1)]"
                  : "font-normal bg-transparent text-[var(--ink-3)]"
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="screen-enter mt-5">
        {activeTab === "sessions" && <SessionsTab />}
        {activeTab === "menu"     && <MenuTab />}
        {activeTab === "tables"   && <TablesTab />}
        {activeTab === "staff"    && <StaffTab />}
        {activeTab === "stats"    && <StatsTab />}
        {activeTab === "plan"     && <PlanTab />}
        {activeTab === "settings" && <SettingsTab />}
      </div>
    </div>
  )
}
