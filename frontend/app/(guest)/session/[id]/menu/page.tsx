"use client"

import { useEffect, useState } from "react"
import { useMenuStore } from "@/store/menu"
import { useCart } from "@/hooks/useCart"
import { useSession } from "@/hooks/useSession"
import { menuApi } from "@/lib/api/menu"
import { formatCurrency } from "@/lib/format"
import { MenuSkeleton } from "@/components/shared/LoadingSkeleton"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Vignette } from "@/components/shared/Vignette"
import { Minus, Plus, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import { useRouter } from "next/navigation"
import { use } from "react"
import type { MenuItem, ItemModifier } from "@/types/api"

interface SheetState {
  item: MenuItem
  quantity: number
  selectedModifiers: number[]
  note: string
}

interface Props {
  params: Promise<{ id: string }>
}

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

export default function MenuPage({ params }: Props) {
  const { id: sessionId } = use(params)
  const router = useRouter()
  const categories = useMenuStore((s) => s.categories)
  const menuLoading = useMenuStore((s) => s.loading)
  const { session } = useSession()
  const { items: cartItems, itemCount, addItem } = useCart()
  const [activeCatId, setActiveCatId] = useState<number | null>(null)
  const [sheet, setSheet] = useState<SheetState | null>(null)
  const [adding, setAdding] = useState(false)

  useEffect(() => {
    if (!session?.branch_id || categories.length > 0) return
    useMenuStore.getState().setLoading(true)
    menuApi
      .getMenu(session.branch_id)
      .then((cats) => useMenuStore.getState().setCategories(cats))
      .catch(() => useMenuStore.getState().setError("Failed to load menu"))
      .finally(() => useMenuStore.getState().setLoading(false))
  }, [session?.branch_id, categories.length])

  useEffect(() => {
    if (categories.length > 0 && activeCatId === null) {
      setActiveCatId(categories[0].id)
    }
  }, [categories.length, activeCatId])

  function openSheet(item: MenuItem) {
    setSheet({ item, quantity: 1, selectedModifiers: [], note: "" })
  }

  function toggleModifier(id: number) {
    setSheet((s) =>
      s ? {
        ...s,
        selectedModifiers: s.selectedModifiers.includes(id)
          ? s.selectedModifiers.filter((m) => m !== id)
          : [...s.selectedModifiers, id],
      } : s
    )
  }

  async function handleAddToCart() {
    if (!sheet) return
    setAdding(true)
    try {
      await addItem(sheet.item.id, sheet.quantity, sheet.selectedModifiers, sheet.note || undefined)
      toast.success(`${sheet.item.name} added`)
      setSheet(null)
    } catch {
      toast.error("Couldn't add item. Please try again.")
    } finally {
      setAdding(false)
    }
  }

  const cartCountByItem: Record<number, number> = {}
  for (const ci of cartItems) {
    cartCountByItem[ci.menu_item_id] = (cartCountByItem[ci.menu_item_id] ?? 0) + ci.quantity
  }

  const cartTotal = cartItems.reduce((s, ci) => s + (ci.item_price ?? 0) * ci.quantity, 0)
  const activeCategory = categories.find((c) => c.id === activeCatId) ?? categories[0]

  if (menuLoading) return <MenuSkeleton />

  return (
    <div className="flex flex-col h-full" style={{ background: "var(--bg-base)", overflow: "hidden" }}>

      {/* Editorial header */}
      <div style={{ padding: "22px 20px 0", flexShrink: 0 }}>
        <span className="eyebrow">The Carte</span>
        <h1 className="serif" style={{ margin: "6px 0 4px", fontSize: 30, fontWeight: 500, letterSpacing: "-0.02em", color: "var(--ink-1)", lineHeight: 1.05 }}>
          Tonight&apos;s menu
        </h1>
        <p style={{ margin: "0 0 16px", color: "var(--ink-3)", fontSize: 12.5 }}>
          {categories.reduce((s, c) => s + c.items.length, 0)} dishes available
        </p>
      </div>

      {/* Category pills */}
      <div className="hscroll" style={{ padding: "0 20px 14px", display: "flex", gap: 6, flexShrink: 0 }}>
        {categories.map((c) => {
          const active = c.id === activeCatId
          return (
            <button
              key={c.id}
              onClick={() => setActiveCatId(c.id)}
              className="press"
              style={{
                padding: "8px 14px", borderRadius: 999, border: "1px solid",
                borderColor: active ? "var(--accent)" : "var(--line-2)",
                background: active ? "var(--accent-soft)" : "var(--bg-elev-1)",
                color: active ? "var(--accent)" : "var(--ink-2)",
                fontSize: 12.5, fontWeight: active ? 600 : 500,
                whiteSpace: "nowrap", boxShadow: active ? "none" : "var(--shadow-1)",
                transition: "all 0.18s var(--ease)",
              }}
            >
              {c.name}
            </button>
          )
        })}
      </div>

      {/* Item list */}
      <div className="scrollarea flex-1 overflow-y-auto" style={{ paddingBottom: itemCount > 0 ? 72 : 16 }}>
        {activeCategory && (
          <div style={{ padding: "0 20px 24px" }}>
            <div className="eyebrow" style={{ marginBottom: 10 }}>
              {activeCategory.name} · {activeCategory.items.length} {activeCategory.items.length === 1 ? "dish" : "dishes"}
            </div>
            <div style={{ display: "flex", flexDirection: "column" }}>
              {activeCategory.items.map((item, idx, arr) => {
                const qty = cartCountByItem[item.id] ?? 0
                return (
                  <div key={item.id}>
                    <button
                      onClick={() => item.is_available && openSheet(item)}
                      disabled={!item.is_available}
                      className="press"
                      style={{
                        border: 0, background: "transparent", padding: "14px 0",
                        display: "flex", gap: 14, alignItems: "flex-start", textAlign: "left",
                        width: "100%", opacity: item.is_available ? 1 : 0.4,
                      }}
                      aria-disabled={!item.is_available}
                    >
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                          <span className="serif" style={{ fontSize: 18, fontWeight: 500, letterSpacing: "-0.01em", color: "var(--ink-1)", lineHeight: 1.2 }}>
                            {item.name}
                          </span>
                          <span className="leader" />
                          <span className="serif" style={{ fontSize: 16, color: "var(--accent)", fontWeight: 500, whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>
                            {formatCurrency(item.price)}
                          </span>
                        </div>
                        {item.description && (
                          <div style={{ color: "var(--ink-3)", fontSize: 12.5, lineHeight: 1.5, marginTop: 4 }}>
                            {item.description}
                          </div>
                        )}
                        {!item.is_available && (
                          <div style={{ color: "var(--ink-4)", fontSize: 11.5, marginTop: 4, letterSpacing: "0.04em", textTransform: "uppercase" }}>
                            Not available
                          </div>
                        )}
                        {qty > 0 && (
                          <div style={{ marginTop: 8, display: "flex", alignItems: "center", gap: 5 }}>
                            <span style={{
                              display: "inline-flex", alignItems: "center", gap: 5,
                              padding: "3px 8px", borderRadius: 999,
                              background: "var(--ok-soft)", color: "var(--ok)",
                              fontSize: 10.5, fontWeight: 600, letterSpacing: "0.02em",
                            }}>
                              ✓ Added · {qty}
                            </span>
                          </div>
                        )}
                      </div>
                      {/* Vignette + qty dot */}
                      <div style={{ position: "relative", flexShrink: 0 }}>
                        <Vignette hue={itemHue(item.id)} size={64} ring={qty > 0} />
                        {qty > 0 && (
                          <span style={{
                            position: "absolute", bottom: -3, right: -3,
                            minWidth: 22, height: 22, borderRadius: 999, padding: "0 7px",
                            background: "var(--accent)", color: "var(--accent-ink)",
                            border: "2px solid var(--bg-base)",
                            display: "inline-flex", alignItems: "center", justifyContent: "center",
                            fontSize: 11, fontWeight: 700, letterSpacing: "-0.005em",
                            fontVariantNumeric: "tabular-nums",
                            boxShadow: "0 4px 12px -4px rgba(0,0,0,0.5)",
                            animation: "pop 0.32s var(--ease-back)",
                          }}>
                            {qty}
                          </span>
                        )}
                      </div>
                    </button>
                    {idx < arr.length - 1 && <hr className="rule" style={{ margin: 0 }} />}
                  </div>
                )
              })}
            </div>
          </div>
        )}
      </div>

      {/* Cart bar */}
      {itemCount > 0 && (
        <div style={{
          padding: "10px 16px 6px", flexShrink: 0,
          background: "color-mix(in srgb, var(--bg-base) 85%, transparent)",
          backdropFilter: "blur(20px)", WebkitBackdropFilter: "blur(20px)",
          borderTop: "1px solid var(--line-1)",
          animation: "slideUp 0.32s var(--ease)",
        }}>
          <button
            onClick={() => router.push(`/session/${sessionId}/cart`)}
            className="press"
            style={{
              width: "100%", height: 52, borderRadius: 14,
              background: "linear-gradient(180deg, var(--accent-strong), var(--accent))",
              border: "1px solid var(--accent)",
              color: "var(--accent-ink)",
              boxShadow: "var(--shadow-2), inset 0 1px 0 rgba(255,255,255,0.25)",
              display: "flex", alignItems: "center", justifyContent: "space-between",
              padding: "0 18px",
            }}
          >
            <span style={{ display: "inline-flex", alignItems: "center", gap: 10, fontSize: 14, fontWeight: 600 }}>
              <span style={{
                width: 24, height: 24, borderRadius: 999,
                background: "rgba(0,0,0,0.18)", color: "inherit",
                display: "inline-flex", alignItems: "center", justifyContent: "center",
                fontSize: 12, fontWeight: 600,
              }}>{itemCount}</span>
              In your cart
            </span>
            <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 14, fontWeight: 600 }}>
              {cartTotal > 0 ? formatCurrency(cartTotal) : ""} <ChevronRight style={{ width: 14, height: 14 }} aria-hidden />
            </span>
          </button>
        </div>
      )}

      {/* Item bottom sheet */}
      <BottomSheet
        open={!!sheet}
        onClose={() => setSheet(null)}
        title={sheet?.item.name}
      >
        {sheet && (
          <div style={{ display: "flex", flexDirection: "column", gap: 20, paddingBottom: 8 }}>
            {/* Description */}
            {sheet.item.description && (
              <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.6 }}>
                {sheet.item.description}
              </p>
            )}

            {/* Price + stepper */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <span className="serif" style={{ fontSize: 24, fontWeight: 500, color: "var(--accent)" }}>
                {formatCurrency(sheet.item.price)}
              </span>
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: Math.max(1, s.quantity - 1) } : s)}
                  style={{
                    width: 36, height: 36, borderRadius: 999,
                    border: "1px solid var(--line-2)", color: "var(--ink-1)",
                    background: "var(--bg-elev-2)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                  }}
                  aria-label="Decrease quantity"
                >
                  <Minus style={{ width: 14, height: 14 }} aria-hidden />
                </button>
                <span style={{ width: 28, textAlign: "center", fontWeight: 600, fontSize: 16, color: "var(--ink-1)" }}>
                  {sheet.quantity}
                </span>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: s.quantity + 1 } : s)}
                  style={{
                    width: 36, height: 36, borderRadius: 999,
                    background: "var(--accent)", color: "var(--accent-ink)",
                    border: "none",
                    display: "flex", alignItems: "center", justifyContent: "center",
                  }}
                  aria-label="Increase quantity"
                >
                  <Plus style={{ width: 14, height: 14 }} aria-hidden />
                </button>
              </div>
            </div>

            {/* Modifiers */}
            {sheet.item.modifiers && sheet.item.modifiers.length > 0 && (
              <div>
                <span className="eyebrow" style={{ display: "block", marginBottom: 10 }}>Customise</span>
                <div style={{ borderRadius: "var(--rad-md)", overflow: "hidden", border: "1px solid var(--line-2)" }}>
                  {sheet.item.modifiers.map((mod: ItemModifier, i: number) => (
                    <label
                      key={mod.id}
                      style={{
                        display: "flex", alignItems: "center", justifyContent: "space-between",
                        padding: "12px 14px", cursor: "default",
                        borderTop: i > 0 ? "1px solid var(--line-1)" : "none",
                        background: sheet.selectedModifiers.includes(mod.id) ? "var(--accent-soft)" : "transparent",
                        transition: "background 0.14s",
                      }}
                    >
                      <span style={{ fontSize: 14, color: "var(--ink-1)" }}>{mod.name}</span>
                      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                        {mod.price_delta !== 0 && (
                          <span style={{ fontSize: 12, color: "var(--ink-3)" }}>
                            +{formatCurrency(mod.price_delta)}
                          </span>
                        )}
                        <input
                          type="checkbox"
                          checked={sheet.selectedModifiers.includes(mod.id)}
                          onChange={() => toggleModifier(mod.id)}
                          style={{ width: 18, height: 18, accentColor: "var(--accent)" }}
                        />
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* Special note */}
            <div>
              <span className="eyebrow" style={{ display: "block", marginBottom: 8 }}>Special note</span>
              <textarea
                value={sheet.note}
                onChange={(e) => setSheet((s) => s ? { ...s, note: e.target.value } : s)}
                placeholder="e.g. No onions, extra spicy…"
                rows={2}
                style={{
                  width: "100%", borderRadius: "var(--rad-md)", padding: "10px 14px",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-1)", fontSize: 14, lineHeight: 1.5,
                  resize: "none", outline: "none",
                }}
              />
            </div>

            {/* CTA */}
            <button
              onClick={handleAddToCart}
              disabled={adding}
              className="press"
              style={{
                width: "100%", height: 52, borderRadius: "var(--rad-md)",
                background: adding ? "var(--bg-elev-3)" : "var(--accent)",
                color: adding ? "var(--ink-3)" : "var(--accent-ink)",
                fontSize: 15, fontWeight: 600,
                border: "none", transition: "background 0.14s",
              }}
            >
              {adding
                ? "Adding…"
                : `Add to cart · ${formatCurrency(
                    (parseFloat(String(sheet.item.price)) +
                      sheet.selectedModifiers.reduce((sum, id) => {
                        const mod = sheet.item.modifiers?.find((m) => m.id === id)
                        return sum + (mod ? parseFloat(String(mod.price_delta)) : 0)
                      }, 0)) * sheet.quantity
                  )}`
              }
            </button>
          </div>
        )}
      </BottomSheet>
    </div>
  )
}
