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
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode, Settings2, ChevronDown, ChevronRight, Plus, Trash2, Edit2, Tag } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, MenuItem, StaffRole, Table, Promo } from "@/types/api"
import { promosApi } from "@/lib/api/promos"
import { tablesApi } from "@/lib/api/tables"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { MenuItemModal } from "@/components/admin/MenuItemModal"
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
                  {s.session_number ?? `Session ${s.id.slice(0, 8)}`}
                </p>
                <p style={{ fontSize: 12, color: "var(--ink-3)" }}>
                  {s.table_identifier ?? `Table ${s.table_id}`} · Visit {s.visit_number ?? "—"} · {relativeTime(s.created_at)}
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

function MenuTab() {
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

// ─── Appearance Tab ──────────────────────────────────────────────────────────

const PRESET_THEMES = [
  {
    id: "dark-luxury",
    name: "Dark Luxury",
    description: "Warm, intimate, evening dining",
    colors: { bg: "#0E0C09", elev: "#1C1914", accent: "#C9A876", ink: "#F4E8D1" },
  },
  {
    id: "modern-minimal",
    name: "Modern Minimal",
    description: "Airy, daytime, casual bistro",
    colors: { bg: "#FAFAF7", elev: "#F0EDE8", accent: "#16140F", ink: "#16140F" },
  },
] as const

type ThemeId = (typeof PRESET_THEMES)[number]["id"]

function AppearanceTab() {
  const { branchId, token, role } = useStaffStore()
  const [selectedTheme, setSelectedTheme] = useState<ThemeId>("dark-luxury")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then(b => { if (b.theme) setSelectedTheme(b.theme as ThemeId) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [branchId, token])

  async function handleSave() {
    if (!branchId || !token) return
    setSaving(true)
    try {
      await staffApi.updateBranch(branchId, { theme: selectedTheme }, token)
      document.documentElement.dataset.theme = selectedTheme
      toast.success("Appearance saved")
    } catch {
      toast.error("Failed to save appearance")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Theme</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 18 }}>
          Your guests will see this theme on their phones when they scan the table QR code.
        </p>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          {PRESET_THEMES.map(theme => {
            const isSelected = selectedTheme === theme.id
            return (
              <button
                key={theme.id}
                onClick={() => canEdit && setSelectedTheme(theme.id)}
                disabled={!canEdit}
                style={{
                  width: 160,
                  background: "none",
                  border: `2px solid ${isSelected ? "var(--accent)" : "var(--line-1)"}`,
                  borderRadius: "var(--rad-md)",
                  padding: 0,
                  cursor: canEdit ? "pointer" : "default",
                  overflow: "hidden",
                  boxShadow: isSelected ? "0 0 0 1px var(--accent)" : "none",
                  transition: "border-color 0.15s, box-shadow 0.15s",
                }}
              >
                {/* Color preview area */}
                <div style={{ background: theme.colors.bg, height: 80, padding: 12, display: "flex", flexDirection: "column", gap: 6 }}>
                  <div style={{ display: "flex", gap: 5 }}>
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.elev }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.accent }} />
                    <div style={{ width: 28, height: 14, borderRadius: 4, background: theme.colors.ink, opacity: 0.7 }} />
                  </div>
                  <div style={{ width: "100%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                  <div style={{ width: "70%", height: 6, borderRadius: 3, background: theme.colors.elev }} />
                </div>
                {/* Label */}
                <div style={{ background: theme.colors.elev, padding: "8px 10px", textAlign: "left" }}>
                  <p style={{ margin: 0, fontSize: 12, fontWeight: 600, color: theme.colors.ink, lineHeight: 1.3 }}>
                    {theme.name}
                  </p>
                  {isSelected && (
                    <p style={{ margin: "2px 0 0", fontSize: 11, color: theme.colors.accent, lineHeight: 1.2 }}>
                      Selected
                    </p>
                  )}
                </div>
              </button>
            )
          })}
        </div>

        {canEdit && (
          <div style={{ marginTop: 20 }}>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? <Loader2 className="size-4 animate-spin" /> : "Save appearance"}
            </Button>
          </div>
        )}
      </HospitalityCard>
    </div>
  )
}

// ─── Settings Tab ────────────────────────────────────────────────────────────

function SettingsTab() {
  const { branchId, token, role } = useStaffStore()
  const [timeoutMinutes, setTimeoutMinutes] = useState(120)
  const [orderPrefix, setOrderPrefix] = useState("OR")
  const [prefixInput, setPrefixInput] = useState("OR")
  const [taxRateInput, setTaxRateInput] = useState("0")
  const [svcRateInput, setSvcRateInput] = useState("0")
  const [includeTaxInPrice, setIncludeTaxInPrice] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingPrefix, setSavingPrefix] = useState(false)
  const [savingBilling, setSavingBilling] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then((b) => {
        setTimeoutMinutes(b.session_timeout_minutes)
        setOrderPrefix(b.order_prefix ?? "OR")
        setPrefixInput(b.order_prefix ?? "OR")
        setTaxRateInput(String(Math.round((b.tax_rate ?? 0) * 100)))
        setSvcRateInput(String(Math.round((b.service_charge_rate ?? 0) * 100)))
        setIncludeTaxInPrice(b.include_tax_in_price ?? false)
      })
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

  async function handleSavePrefix() {
    if (!branchId || !token) return
    const p = prefixInput.toUpperCase().trim()
    if (!/^[A-Z]{2,4}$/.test(p)) {
      toast.error("Prefix must be 2–4 uppercase letters")
      return
    }
    setSavingPrefix(true)
    try {
      await staffApi.updateBranch(branchId, { order_prefix: p }, token)
      setOrderPrefix(p)
      toast.success("Order prefix updated")
    } catch {
      toast.error("Failed to update prefix")
    } finally {
      setSavingPrefix(false)
    }
  }

  async function handleSaveBilling() {
    if (!branchId || !token) return
    const taxRate = parseFloat(taxRateInput) / 100
    const svcRate = parseFloat(svcRateInput) / 100
    if (isNaN(taxRate) || taxRate < 0 || taxRate > 0.5) {
      toast.error("Tax rate must be between 0% and 50%")
      return
    }
    if (isNaN(svcRate) || svcRate < 0 || svcRate > 0.5) {
      toast.error("Service charge must be between 0% and 50%")
      return
    }
    setSavingBilling(true)
    try {
      await staffApi.updateBranch(branchId, {
        tax_rate: taxRate,
        service_charge_rate: svcRate,
        include_tax_in_price: includeTaxInPrice,
      }, token)
      toast.success("Billing settings saved")
    } catch {
      toast.error("Failed to save billing settings")
    } finally {
      setSavingBilling(false)
    }
  }

  const taxRateNum = parseFloat(taxRateInput) || 0
  const billingPreview = taxRateNum > 0 && !includeTaxInPrice
    ? `An order of ₹100 will show as: ₹100 + ₹${taxRateNum} GST = ₹${100 + taxRateNum} total`
    : includeTaxInPrice
    ? "Menu prices already include tax — no additional tax is added."
    : "No tax is added to orders."

  if (loading) return null

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
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

      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Order numbers</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
          A short prefix added to every order number. Example: prefix <span className="mono">TM</span> → <span className="mono">#TM1001</span>.
          Counters reset daily. Changes take effect on the next order.
        </p>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <Input
            type="text"
            maxLength={4}
            value={prefixInput}
            onChange={(e) => setPrefixInput(e.target.value.toUpperCase())}
            disabled={!canEdit}
            style={{ width: 72, fontFamily: "var(--font-mono, monospace)", letterSpacing: "0.08em", textTransform: "uppercase" }}
            placeholder="OR"
          />
          {canEdit && (
            <Button
              onClick={handleSavePrefix}
              disabled={savingPrefix}
            >
              {savingPrefix ? <Loader2 className="size-4 animate-spin" /> : "Save"}
            </Button>
          )}
        </div>
        <p className="text-sm mono" style={{ marginTop: 10, color: "var(--ink-3)" }}>
          Preview: #{orderPrefix}1001, #{orderPrefix}1002, #{orderPrefix}1003&hellip;
        </p>
      </HospitalityCard>

      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Billing</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
          Tax and service charge settings shown on the itemized bill.
        </p>

        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span className="text-sm" style={{ color: "var(--ink-2)", width: 140 }}>Tax rate (GST %)</span>
            <Input
              type="number"
              min={0}
              max={50}
              step={1}
              value={taxRateInput}
              onChange={(e) => setTaxRateInput(e.target.value)}
              disabled={!canEdit}
              style={{ width: 72 }}
            />
            <span className="text-sm" style={{ color: "var(--ink-3)" }}>%</span>
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span className="text-sm" style={{ color: "var(--ink-2)", width: 140 }}>Service charge %</span>
            <Input
              type="number"
              min={0}
              max={50}
              step={1}
              value={svcRateInput}
              onChange={(e) => setSvcRateInput(e.target.value)}
              disabled={!canEdit}
              style={{ width: 72 }}
            />
            <span className="text-sm" style={{ color: "var(--ink-3)" }}>%</span>
          </div>

          <label style={{ display: "flex", alignItems: "center", gap: 10, cursor: canEdit ? "pointer" : "default" }}>
            <input
              type="checkbox"
              checked={includeTaxInPrice}
              onChange={(e) => setIncludeTaxInPrice(e.target.checked)}
              disabled={!canEdit}
              style={{ width: 16, height: 16, accentColor: "var(--accent)" }}
            />
            <span className="text-sm" style={{ color: "var(--ink-2)" }}>
              Prices include tax (do not add tax on top)
            </span>
          </label>
        </div>

        <p className="text-sm" style={{ marginTop: 12, color: "var(--ink-3)", fontStyle: "italic", lineHeight: 1.5 }}>
          {billingPreview}
        </p>

        {canEdit && (
          <Button
            onClick={handleSaveBilling}
            disabled={savingBilling}
            style={{ marginTop: 14 }}
          >
            {savingBilling ? <Loader2 className="size-4 animate-spin" /> : "Save billing settings"}
          </Button>
        )}
      </HospitalityCard>
    </div>
  )
}

// ─── Promos Tab ──────────────────────────────────────────────────────────────

function PromosTab() {
  const { branchId, token } = useStaffStore()
  const [promos, setPromos] = useState<Promo[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)

  // Create form state
  const [code, setCode] = useState("")
  const [type, setType] = useState<"flat_amount" | "percentage">("percentage")
  const [value, setValue] = useState("")
  const [minOrder, setMinOrder] = useState("")
  const [maxUses, setMaxUses] = useState("")
  const [validFrom, setValidFrom] = useState("")
  const [validUntil, setValidUntil] = useState("")
  const [windowStart, setWindowStart] = useState("")
  const [windowEnd, setWindowEnd] = useState("")
  const [description, setDescription] = useState("")

  const fetchPromos = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await promosApi.list(branchId, token)
      setPromos(data)
    } catch {
      toast.error("Couldn't load promos.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => { fetchPromos() }, [fetchPromos])

  async function handleCreate() {
    if (!branchId || !token) return
    if (!code.trim() || !value || !validFrom || !validUntil) {
      toast.error("Code, value, and date range are required.")
      return
    }
    setCreating(true)
    try {
      await promosApi.create(branchId, {
        code: code.trim().toUpperCase(),
        type,
        value: parseFloat(value),
        min_order_amount: minOrder ? parseFloat(minOrder) : 0,
        max_uses: maxUses ? parseInt(maxUses) : null,
        uses_per_phone: 1,
        valid_from: new Date(validFrom).toISOString(),
        valid_until: new Date(validUntil).toISOString(),
        time_window_start: windowStart || null,
        time_window_end: windowEnd || null,
        description: description || null,
      }, token)
      toast.success("Promo created.")
      setShowCreate(false)
      setCode(""); setValue(""); setMinOrder(""); setMaxUses("")
      setValidFrom(""); setValidUntil(""); setWindowStart(""); setWindowEnd(""); setDescription("")
      fetchPromos()
    } catch {
      toast.error("Failed to create promo. Check the code isn't already in use.")
    } finally {
      setCreating(false)
    }
  }

  async function handleDeactivate(promoId: number) {
    if (!branchId || !token) return
    try {
      await promosApi.deactivate(branchId, promoId, token)
      toast.success("Promo deactivated.")
      fetchPromos()
    } catch {
      toast.error("Failed to deactivate promo.")
    }
  }

  function formatPromoValue(p: Promo) {
    return p.type === "percentage" ? `${p.value}% off` : `₹${p.value} off`
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
          {promos.length} promo{promos.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={() => setShowCreate((v) => !v)}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6,
            fontSize: 12, color: showCreate ? "var(--accent)" : "var(--ink-3)",
            padding: "6px 10px", borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
          }}
        >
          <Plus size={12} aria-hidden />
          New promo
        </button>
      </div>

      {showCreate && (
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", flexDirection: "column", gap: 12 }}>
          <p className="eyebrow" style={{ marginBottom: 4 }}>Create promotion</p>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Code</label>
              <Input
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="HAPPY20"
              />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Type</label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as "flat_amount" | "percentage")}
                style={{
                  width: "100%", height: 36, borderRadius: "var(--rad-md)",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-1)", fontSize: 13, padding: "0 8px",
                }}
              >
                <option value="percentage">Percentage %</option>
                <option value="flat_amount">Flat ₹</option>
              </select>
            </div>
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>
                Value ({type === "percentage" ? "%" : "₹"})
              </label>
              <Input type="number" value={value} onChange={(e) => setValue(e.target.value)} placeholder="20" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Min order (₹)</label>
              <Input type="number" value={minOrder} onChange={(e) => setMinOrder(e.target.value)} placeholder="0" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Max uses</label>
              <Input type="number" value={maxUses} onChange={(e) => setMaxUses(e.target.value)} placeholder="∞" />
            </div>
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Valid from</label>
              <Input type="datetime-local" value={validFrom} onChange={(e) => setValidFrom(e.target.value)} />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Valid until</label>
              <Input type="datetime-local" value={validUntil} onChange={(e) => setValidUntil(e.target.value)} />
            </div>
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Time window start (HH:MM)</label>
              <Input type="time" value={windowStart} onChange={(e) => setWindowStart(e.target.value)} placeholder="16:00" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Time window end (HH:MM)</label>
              <Input type="time" value={windowEnd} onChange={(e) => setWindowEnd(e.target.value)} placeholder="19:00" />
            </div>
          </div>

          <div>
            <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Description (shown to guests)</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Happy Hour — 20% off all beverages"
            />
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <Button onClick={handleCreate} disabled={creating} style={{ flex: 1 }}>
              {creating ? <Loader2 className="size-4 animate-spin" /> : "Create promo"}
            </Button>
            <Button
              onClick={() => setShowCreate(false)}
              style={{ flex: 1, background: "var(--bg-elev-2)", color: "var(--ink-2)", border: "1px solid var(--line-1)" }}
            >
              Cancel
            </Button>
          </div>
        </HospitalityCard>
      )}

      {promos.length === 0 ? (
        <EmptyState
          icon={Tag}
          title="No promos yet"
          description="Create a promo code to offer discounts to guests."
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {promos.map((p) => (
            <HospitalityCard
              key={p.id}
              elev={1}
              style={{
                padding: "14px 16px",
                opacity: p.is_active ? 1 : 0.5,
              }}
            >
              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                    <p className="mono" style={{ fontSize: 13, fontWeight: 700, color: "var(--accent)", letterSpacing: "0.05em" }}>
                      {p.code}
                    </p>
                    <span style={{
                      fontSize: 11, fontWeight: 500, padding: "2px 8px",
                      borderRadius: "var(--rad-pill)",
                      background: p.is_active ? "var(--ok-soft)" : "var(--bg-elev-2)",
                      color: p.is_active ? "var(--ok)" : "var(--ink-3)",
                      border: `1px solid ${p.is_active ? "var(--ok)" : "var(--line-1)"}`,
                    }}>
                      {p.is_active ? "Active" : "Inactive"}
                    </span>
                  </div>
                  <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)", marginBottom: 2 }}>
                    {formatPromoValue(p)}
                    {p.min_order_amount > 0 && ` · min ₹${p.min_order_amount}`}
                  </p>
                  {p.description && (
                    <p style={{ fontSize: 12, color: "var(--ink-3)", marginBottom: 4, lineHeight: 1.4 }}>{p.description}</p>
                  )}
                  <p style={{ fontSize: 11, color: "var(--ink-4)" }}>
                    {new Date(p.valid_from).toLocaleDateString()} – {new Date(p.valid_until).toLocaleDateString()}
                    {p.max_uses && ` · max ${p.max_uses} uses`}
                    {p.time_window_start && ` · ${p.time_window_start}–${p.time_window_end}`}
                  </p>
                </div>
                {p.is_active && (
                  <button
                    onClick={() => handleDeactivate(p.id)}
                    className="press"
                    style={{
                      flexShrink: 0, fontSize: 12, color: "var(--ink-3)",
                      padding: "5px 10px", borderRadius: "var(--rad-md)",
                      background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
                    }}
                  >
                    Deactivate
                  </button>
                )}
              </div>
            </HospitalityCard>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Page ────────────────────────────────────────────────────────────────────

const TABS = [
  { id: "sessions",   label: "Sessions"   },
  { id: "menu",       label: "Menu"       },
  { id: "tables",     label: "Tables"     },
  { id: "staff",      label: "Staff"      },
  { id: "stats",      label: "Stats"      },
  { id: "plan",       label: "Plan"       },
  { id: "promos",     label: "Promos"     },
  { id: "appearance", label: "Appearance" },
  { id: "settings",   label: "Settings"   },
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
        {activeTab === "sessions"   && <SessionsTab />}
        {activeTab === "menu"       && <MenuTab />}
        {activeTab === "tables"     && <TablesTab />}
        {activeTab === "staff"      && <StaffTab />}
        {activeTab === "stats"      && <StatsTab />}
        {activeTab === "plan"       && <PlanTab />}
        {activeTab === "promos"     && <PromosTab />}
        {activeTab === "appearance" && <AppearanceTab />}
        {activeTab === "settings"   && <SettingsTab />}
      </div>
    </div>
  )
}
