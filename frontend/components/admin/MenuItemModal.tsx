"use client"

import { useState } from "react"
import { Loader2, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"
import { staffApi } from "@/lib/api/staff"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { MenuItem, MenuCategory, ItemModifier, DietaryFlag, ItemBadge } from "@/types/api"

// ── Constants ────────────────────────────────────────────────────────────────

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

// ── Types ─────────────────────────────────────────────────────────────────────

interface PendingModifier {
  name: string
  modifier_group: string
  price_delta: string
  is_required: boolean
}

interface ModalForm {
  name: string
  description: string
  price: string
  position: number
  categoryId: number
  dietaryFlags: DietaryFlag[]
  itemBadges: ItemBadge[]
  spiceLevel: number
  isAvailable: boolean
  isFeatured: boolean
  featuredSortOrder: number
  modifiers: ItemModifier[]
  pendingModifiers: PendingModifier[]
  deletedModifierIds: number[]
  newMod: PendingModifier
  addingMod: boolean
  saving: boolean
  deleting: boolean
  confirmDelete: boolean
  errors: { name?: string; price?: string; categoryId?: string }
}

export interface MenuItemModalProps {
  mode: "create" | "edit"
  item?: MenuItem
  categories: MenuCategory[]
  defaultCategoryId?: number
  branchId: number
  token: string
  onClose: () => void
  onSaved: (item: MenuItem) => void
  onDeleted?: (itemId: number) => void
}

// ── Component ─────────────────────────────────────────────────────────────────

export function MenuItemModal({
  mode, item, categories, defaultCategoryId, branchId, token, onClose, onSaved, onDeleted,
}: MenuItemModalProps) {
  const initialCategoryId = item?.category_id ?? defaultCategoryId ?? categories[0]?.id ?? 0

  const [form, setForm] = useState<ModalForm>({
    name: item?.name ?? "",
    description: item?.description ?? "",
    price: item?.price ?? "",
    position: item?.position ?? 0,
    categoryId: initialCategoryId,
    dietaryFlags: item?.dietary_flags ?? [],
    itemBadges: item?.item_badges ?? [],
    spiceLevel: item?.spice_level ?? 0,
    isAvailable: item?.is_available ?? true,
    isFeatured: item?.is_featured ?? false,
    featuredSortOrder: item?.featured_sort_order ?? 0,
    modifiers: item?.modifiers ?? [],
    pendingModifiers: [],
    deletedModifierIds: [],
    newMod: { name: "", modifier_group: "", price_delta: "0", is_required: false },
    addingMod: false,
    saving: false,
    deleting: false,
    confirmDelete: false,
    errors: {},
  })

  function patch(updates: Partial<ModalForm>) {
    setForm((f) => ({ ...f, ...updates }))
  }

  // ── Validation ──────────────────────────────────────────────────────────────

  function validate(): boolean {
    const errors: ModalForm["errors"] = {}
    if (!form.name.trim()) errors.name = "Name is required"
    else if (form.name.trim().length > 80) errors.name = "Name must be 80 chars or fewer"
    const priceNum = parseFloat(form.price.replace(",", "."))
    if (isNaN(priceNum) || priceNum < 0) errors.price = "Price must be ≥ 0"
    if (!form.categoryId) errors.categoryId = "Category is required"
    if (Object.keys(errors).length) { patch({ errors }); return false }
    return true
  }

  // ── Save ────────────────────────────────────────────────────────────────────

  async function handleSave() {
    if (!validate()) return
    patch({ saving: true, errors: {} })
    const priceNum = parseFloat(form.price.replace(",", "."))

    try {
      let saved: MenuItem

      if (mode === "create") {
        saved = await staffApi.createMenuItem(
          branchId,
          form.categoryId,
          form.name.trim(),
          priceNum,
          token,
          {
            description: form.description,
            is_available: form.isAvailable,
            dietary_flags: form.dietaryFlags,
            item_badges: form.itemBadges,
            spice_level: form.spiceLevel,
          }
        )

        if (form.isFeatured) {
          await staffApi.toggleFeatured(saved.id, branchId, true, form.featuredSortOrder, token)
          saved = { ...saved, is_featured: true, featured_sort_order: form.featuredSortOrder }
        }

        for (const mod of form.pendingModifiers) {
          const created = await staffApi.addModifier(saved.id, branchId, {
            name: mod.name.trim(),
            price_delta: parseFloat(mod.price_delta) || 0,
            is_required: mod.is_required,
            modifier_group: mod.modifier_group.trim(),
          }, token)
          saved = { ...saved, modifiers: [...(saved.modifiers ?? []), created] }
        }
      } else {
        // edit mode
        saved = await staffApi.updateMenuItem(
          item!.id,
          branchId,
          {
            name: form.name.trim(),
            price: priceNum,
            description: form.description,
            position: form.position,
            dietary_flags: form.dietaryFlags,
            item_badges: form.itemBadges,
            spice_level: form.spiceLevel,
            category_id: form.categoryId !== item!.category_id ? form.categoryId : undefined,
          },
          token
        )

        const featuredChanged =
          form.isFeatured !== (item!.is_featured ?? false) ||
          form.featuredSortOrder !== (item!.featured_sort_order ?? 0)
        if (featuredChanged) {
          await staffApi.toggleFeatured(saved.id, branchId, form.isFeatured, form.featuredSortOrder, token)
          saved = { ...saved, is_featured: form.isFeatured, featured_sort_order: form.featuredSortOrder }
        } else {
          saved = { ...saved, is_featured: item!.is_featured, featured_sort_order: item!.featured_sort_order }
        }

        for (const modId of form.deletedModifierIds) {
          await staffApi.deleteModifier(modId, branchId, token)
        }

        const remainingMods = form.modifiers.filter((m) => !form.deletedModifierIds.includes(m.id))
        const addedMods: ItemModifier[] = []
        for (const mod of form.pendingModifiers) {
          const created = await staffApi.addModifier(saved.id, branchId, {
            name: mod.name.trim(),
            price_delta: parseFloat(mod.price_delta) || 0,
            is_required: mod.is_required,
            modifier_group: mod.modifier_group.trim(),
          }, token)
          addedMods.push(created)
        }
        saved = { ...saved, modifiers: [...remainingMods, ...addedMods] }
      }

      onSaved(saved)
      onClose()
    } catch {
      toast.error("Couldn't save item. Please try again.")
      patch({ saving: false })
    }
  }

  // ── Delete ──────────────────────────────────────────────────────────────────

  async function handleDelete() {
    if (!item) return
    patch({ deleting: true })
    try {
      await staffApi.deleteMenuItem(item.id, branchId, token)
      onDeleted?.(item.id)
      onClose()
    } catch {
      toast.error("Couldn't delete item. It may have active orders.")
      patch({ deleting: false, confirmDelete: false })
    }
  }

  // ── Modifier helpers ────────────────────────────────────────────────────────

  function addPendingModifier() {
    if (!form.newMod.name.trim()) { toast.error("Modifier name required"); return }
    patch({
      pendingModifiers: [...form.pendingModifiers, form.newMod],
      newMod: { name: "", modifier_group: "", price_delta: "0", is_required: false },
      addingMod: false,
    })
  }

  function removePendingModifier(idx: number) {
    patch({ pendingModifiers: form.pendingModifiers.filter((_, i) => i !== idx) })
  }

  function markModifierDeleted(modId: number) {
    patch({ deletedModifierIds: [...form.deletedModifierIds, modId] })
  }

  // ── Pill button style helper ─────────────────────────────────────────────────

  function pillStyle(active: boolean): React.CSSProperties {
    return {
      fontSize: 12, fontWeight: 600, padding: "7px 14px",
      borderRadius: "var(--rad-pill)", border: "1px solid",
      borderColor: active ? "var(--accent)" : "var(--line-2)",
      background: active ? "var(--accent-soft)" : "var(--bg-elev-2)",
      color: active ? "var(--accent)" : "var(--ink-3)",
      cursor: "pointer",
    }
  }

  const title = mode === "create" ? "Add Item" : `Edit: ${item?.name}`
  const allMods = form.modifiers.filter((m) => !form.deletedModifierIds.includes(m.id))

  return (
    <BottomSheet open={true} onClose={onClose} title={title}>
      <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>

        {/* ── BASICS ─────────────────────────────────────────────────────── */}
        <div style={{ paddingBottom: 20 }}>
          <p className="eyebrow" style={{ marginBottom: 14, color: "var(--ink-3)" }}>Basics</p>

          {/* Name */}
          <div style={{ marginBottom: 12 }}>
            <label style={labelStyle}>Name *</label>
            <Input
              value={form.name}
              maxLength={80}
              autoFocus
              onChange={(e) => patch({ name: e.target.value, errors: { ...form.errors, name: undefined } })}
              style={{ borderColor: form.errors.name ? "var(--err)" : undefined }}
              placeholder="e.g. Chicken Tikka Masala"
            />
            {form.errors.name && <p style={errStyle}>{form.errors.name}</p>}
          </div>

          {/* Category */}
          <div style={{ marginBottom: 12 }}>
            <label style={labelStyle}>Category *</label>
            {categories.length === 0 ? (
              <p style={{ fontSize: 13, color: "var(--warn)" }}>No categories — create a category first</p>
            ) : (
              <select
                value={form.categoryId}
                onChange={(e) => patch({ categoryId: parseInt(e.target.value), errors: { ...form.errors, categoryId: undefined } })}
                style={selectStyle}
              >
                {categories.map((cat) => (
                  <option key={cat.id} value={cat.id}>{cat.name}</option>
                ))}
              </select>
            )}
            {form.errors.categoryId && <p style={errStyle}>{form.errors.categoryId}</p>}
          </div>

          {/* Price + Position */}
          <div style={{ display: "flex", gap: 12, marginBottom: 12 }}>
            <div style={{ flex: 2 }}>
              <label style={labelStyle}>Price (₹) *</label>
              <Input
                type="number"
                min={0}
                step={0.5}
                value={form.price}
                onChange={(e) => patch({ price: e.target.value, errors: { ...form.errors, price: undefined } })}
                style={{ borderColor: form.errors.price ? "var(--err)" : undefined }}
                placeholder="0"
              />
              {form.errors.price && <p style={errStyle}>{form.errors.price}</p>}
            </div>
            <div style={{ flex: 1 }}>
              <label style={labelStyle}>Sort</label>
              <Input
                type="number"
                min={0}
                value={form.position}
                onChange={(e) => patch({ position: parseInt(e.target.value) || 0 })}
              />
            </div>
          </div>

          {/* Description */}
          <div style={{ marginBottom: 12 }}>
            <label style={labelStyle}>Description</label>
            <textarea
              value={form.description}
              rows={2}
              onChange={(e) => patch({ description: e.target.value })}
              placeholder="Short description (optional)"
              style={textareaStyle}
            />
          </div>

          {/* Image placeholder (Plan 02 dependency) */}
          <div style={{
            padding: "10px 14px", borderRadius: "var(--rad-md)",
            border: "1.5px dashed var(--line-2)",
            fontSize: 12, color: "var(--ink-4)", textAlign: "center",
          }}>
            Image upload coming soon
          </div>
        </div>

        {/* ── IDENTITY ───────────────────────────────────────────────────── */}
        <div style={{ borderTop: "1px solid var(--line-1)", paddingTop: 20, paddingBottom: 20 }}>
          <p className="eyebrow" style={{ marginBottom: 14, color: "var(--ink-3)" }}>Identity</p>

          {/* Dietary */}
          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Dietary</label>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 6 }}>
              {DIETARY_OPTIONS.map(({ flag, label }) => {
                const active = form.dietaryFlags.includes(flag)
                return (
                  <button
                    key={flag}
                    type="button"
                    className="press"
                    onClick={() => patch({ dietaryFlags: applyDietaryLogic(form.dietaryFlags, flag, !active) })}
                    style={pillStyle(active)}
                    aria-pressed={active}
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Badges */}
          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Badges</label>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 6 }}>
              {BADGE_OPTIONS.map(({ badge, label }) => {
                const active = form.itemBadges.includes(badge)
                return (
                  <button
                    key={badge}
                    type="button"
                    className="press"
                    onClick={() => patch({
                      itemBadges: active ? form.itemBadges.filter((b) => b !== badge) : [...form.itemBadges, badge],
                    })}
                    style={pillStyle(active)}
                    aria-pressed={active}
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Spice */}
          <div>
            <label style={labelStyle}>Spice Level</label>
            <div style={{ display: "flex", gap: 8, marginTop: 6 }}>
              {SPICE_LABELS.map((label, level) => {
                const active = form.spiceLevel === level
                return (
                  <button
                    key={level}
                    type="button"
                    className="press"
                    onClick={() => patch({ spiceLevel: level })}
                    style={{ ...pillStyle(active), flex: 1, padding: "7px 4px" }}
                    aria-pressed={active}
                  >
                    {label}
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        {/* ── VISIBILITY ─────────────────────────────────────────────────── */}
        <div style={{ borderTop: "1px solid var(--line-1)", paddingTop: 20, paddingBottom: 20 }}>
          <p className="eyebrow" style={{ marginBottom: 14, color: "var(--ink-3)" }}>Visibility</p>

          {/* Available toggle */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
            <span style={{ fontSize: 14, color: "var(--ink-2)" }}>Available to order</span>
            <button
              type="button"
              className="press"
              onClick={() => patch({ isAvailable: !form.isAvailable })}
              style={{
                fontSize: 11, fontWeight: 600, padding: "5px 14px",
                borderRadius: "var(--rad-pill)", border: "none",
                background: form.isAvailable ? "var(--ok)" : "var(--bg-elev-3)",
                color: form.isAvailable ? "white" : "var(--ink-3)",
                cursor: "pointer", minWidth: 46,
              }}
              aria-pressed={form.isAvailable}
            >
              {form.isAvailable ? "On" : "Off"}
            </button>
          </div>

          {/* Featured toggle */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <span style={{ fontSize: 14, color: "var(--ink-2)" }}>Featured in carousel</span>
            <button
              type="button"
              className="press"
              onClick={() => patch({ isFeatured: !form.isFeatured })}
              style={{
                fontSize: 11, fontWeight: 600, padding: "5px 14px",
                borderRadius: "var(--rad-pill)", border: "none",
                background: form.isFeatured ? "var(--accent)" : "var(--bg-elev-3)",
                color: form.isFeatured ? "white" : "var(--ink-3)",
                cursor: "pointer", minWidth: 46,
              }}
              aria-pressed={form.isFeatured}
            >
              {form.isFeatured ? "On" : "Off"}
            </button>
          </div>

          {form.isFeatured && (
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 12 }}>
              <span style={{ fontSize: 12, color: "var(--ink-3)" }}>Featured sort order</span>
              <Input
                type="number"
                min={0}
                value={form.featuredSortOrder}
                onChange={(e) => patch({ featuredSortOrder: parseInt(e.target.value) || 0 })}
                style={{ width: 80 }}
              />
            </div>
          )}
        </div>

        {/* ── MODIFIERS ──────────────────────────────────────────────────── */}
        <div style={{ borderTop: "1px solid var(--line-1)", paddingTop: 20, paddingBottom: 24 }}>
          <p className="eyebrow" style={{ marginBottom: 14, color: "var(--ink-3)" }}>Modifiers</p>

          {/* Existing modifiers (edit mode) */}
          <div style={{ display: "flex", flexDirection: "column", gap: 6, maxHeight: 220, overflowY: "auto" }}>
            {allMods.map((mod) => (
              <div key={mod.id} style={modRowStyle}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)" }}>{mod.name}</p>
                  <p style={{ fontSize: 11, color: "var(--ink-3)" }}>
                    {mod.modifier_group && <span style={{ marginRight: 8 }}>{mod.modifier_group}</span>}
                    {mod.price_delta >= 0 ? `+₹${mod.price_delta}` : `-₹${Math.abs(mod.price_delta)}`}
                    {mod.is_required && <span style={{ marginLeft: 8, color: "var(--accent)" }}>Required</span>}
                  </p>
                </div>
                <button
                  type="button"
                  className="press"
                  onClick={() => markModifierDeleted(mod.id)}
                  style={iconBtnStyle}
                  aria-label="Remove modifier"
                  disabled={form.saving}
                >
                  <Trash2 size={12} />
                </button>
              </div>
            ))}

            {/* Pending modifiers */}
            {form.pendingModifiers.map((mod, idx) => (
              <div key={`pending-${idx}`} style={{ ...modRowStyle, borderColor: "var(--accent-soft)" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)" }}>{mod.name}</p>
                  <p style={{ fontSize: 11, color: "var(--ink-3)" }}>
                    {mod.modifier_group && <span style={{ marginRight: 8 }}>{mod.modifier_group}</span>}
                    {parseFloat(mod.price_delta) >= 0 ? `+₹${mod.price_delta}` : `-₹${Math.abs(parseFloat(mod.price_delta))}`}
                    {mod.is_required && <span style={{ marginLeft: 8, color: "var(--accent)" }}>Required</span>}
                  </p>
                </div>
                <button
                  type="button"
                  className="press"
                  onClick={() => removePendingModifier(idx)}
                  style={iconBtnStyle}
                  aria-label="Remove modifier"
                  disabled={form.saving}
                >
                  <Trash2 size={12} />
                </button>
              </div>
            ))}
          </div>

          {/* Add modifier inline form */}
          {form.addingMod ? (
            <div style={{
              marginTop: 10, display: "flex", flexDirection: "column", gap: 8,
              padding: "12px", background: "var(--bg-elev-2)",
              borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
            }}>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
                <Input
                  placeholder="Name (e.g. Large)"
                  value={form.newMod.name}
                  onChange={(e) => patch({ newMod: { ...form.newMod, name: e.target.value } })}
                  style={{ fontSize: 13 }}
                />
                <Input
                  placeholder="Group (e.g. size)"
                  value={form.newMod.modifier_group}
                  onChange={(e) => patch({ newMod: { ...form.newMod, modifier_group: e.target.value } })}
                  style={{ fontSize: 13 }}
                />
              </div>
              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <Input
                  type="number"
                  placeholder="₹ delta"
                  value={form.newMod.price_delta}
                  onChange={(e) => patch({ newMod: { ...form.newMod, price_delta: e.target.value } })}
                  style={{ flex: 1, fontSize: 13 }}
                />
                <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--ink-2)", cursor: "pointer", flexShrink: 0 }}>
                  <input
                    type="checkbox"
                    checked={form.newMod.is_required}
                    onChange={(e) => patch({ newMod: { ...form.newMod, is_required: e.target.checked } })}
                    style={{ accentColor: "var(--accent)" }}
                  />
                  Required
                </label>
              </div>
              <div style={{ display: "flex", gap: 8 }}>
                <Button onClick={addPendingModifier} style={{ flex: 1 }} disabled={form.saving}>
                  Add
                </Button>
                <Button variant="outline" onClick={() => patch({ addingMod: false })} style={{ flex: 1 }}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              className="press"
              onClick={() => patch({ addingMod: true })}
              disabled={form.saving}
              style={{
                marginTop: allMods.length > 0 || form.pendingModifiers.length > 0 ? 8 : 0,
                width: "100%", padding: "9px 12px",
                borderRadius: "var(--rad-md)", border: "1px dashed var(--line-2)",
                background: "transparent", fontSize: 13, color: "var(--ink-3)",
                cursor: "pointer", display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
              }}
            >
              <Plus size={12} /> Add modifier
            </button>
          )}
        </div>

        {/* ── FOOTER ─────────────────────────────────────────────────────── */}
        <div style={{ borderTop: "1px solid var(--line-1)", paddingTop: 16 }}>

          {/* Delete confirmation (edit mode) */}
          {mode === "edit" && form.confirmDelete && (
            <div
              role="alert"
              style={{
                marginBottom: 12, padding: "10px 14px",
                borderRadius: "var(--rad-md)", background: "var(--bg-elev-2)",
                border: "1px solid var(--line-2)", fontSize: 13, color: "var(--ink-2)",
              }}
            >
              <p style={{ marginBottom: 10 }}>Delete this item permanently?</p>
              <div style={{ display: "flex", gap: 8 }}>
                <Button
                  variant="destructive"
                  onClick={handleDelete}
                  disabled={form.deleting}
                  style={{ flex: 1 }}
                >
                  {form.deleting ? <Loader2 size={14} className="animate-spin" /> : "Delete"}
                </Button>
                <Button variant="outline" onClick={() => patch({ confirmDelete: false })} style={{ flex: 1 }}>
                  Cancel
                </Button>
              </div>
            </div>
          )}

          {/* Action row */}
          <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
            {mode === "edit" && !form.confirmDelete && (
              <button
                type="button"
                className="press"
                onClick={() => patch({ confirmDelete: true })}
                disabled={form.saving}
                style={{
                  fontSize: 13, padding: "0 4px",
                  background: "transparent", border: "none",
                  color: "var(--err)", cursor: "pointer", flexShrink: 0,
                }}
              >
                Delete item
              </button>
            )}
            <button
              type="button"
              onClick={handleSave}
              disabled={form.saving || categories.length === 0}
              className="press btn-primary"
              style={{
                flex: 1, height: 50, borderRadius: 12,
                display: "flex", alignItems: "center", justifyContent: "center",
                fontSize: 15, fontWeight: 700,
                opacity: form.saving ? 0.6 : 1,
                cursor: form.saving || categories.length === 0 ? "not-allowed" : "pointer",
              }}
            >
              {form.saving
                ? <Loader2 size={16} className="animate-spin" />
                : mode === "create" ? "Add Item" : "Save Changes"
              }
            </button>
          </div>
        </div>
      </div>
    </BottomSheet>
  )
}

// ── Shared inline styles ───────────────────────────────────────────────────────

const labelStyle: React.CSSProperties = {
  display: "block", fontSize: 12, fontWeight: 600,
  color: "var(--ink-3)", marginBottom: 6,
}

const errStyle: React.CSSProperties = {
  fontSize: 11, color: "var(--err)", marginTop: 4,
}

const selectStyle: React.CSSProperties = {
  width: "100%", fontSize: 14, padding: "8px 12px",
  borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-2)", color: "var(--ink-1)",
}

const textareaStyle: React.CSSProperties = {
  width: "100%", fontSize: 14, padding: "8px 12px",
  borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-2)", color: "var(--ink-1)", resize: "none",
}

const modRowStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 8,
  padding: "8px 12px", borderRadius: "var(--rad-md)",
  background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
}

const iconBtnStyle: React.CSSProperties = {
  minHeight: 28, minWidth: 28,
  display: "flex", alignItems: "center", justifyContent: "center",
  borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-3)", color: "var(--err)", cursor: "pointer",
  flexShrink: 0,
}
