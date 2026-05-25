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
import { cn } from "@/lib/utils"
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

  const cartTotal = cartItems.reduce((s, ci) => {
    const modTotal = ci.selected_modifiers.reduce((m, mod) => m + mod.price_delta, 0)
    return s + ((ci.item_price ?? 0) + modTotal) * ci.quantity
  }, 0)
  const activeCategory = categories.find((c) => c.id === activeCatId) ?? categories[0]
  const totalDishes = categories.reduce((s, c) => s + c.items.length, 0)

  if (menuLoading) return <MenuSkeleton />

  return (
    <div className="flex flex-col h-full screen-enter" style={{ background: "var(--bg-base)", overflow: "hidden" }}>

      {/* Editorial header with warm glow */}
      <div className="page-glow" style={{ padding: "24px 20px 0", flexShrink: 0 }}>
        <span className="eyebrow">The Carte</span>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          Tonight&apos;s menu
        </h1>
        <p style={{ margin: "0 0 18px", color: "var(--ink-2)", fontSize: 13 }}>
          {totalDishes} dishes available
        </p>
      </div>

      {/* Category pills — edge-faded scroll */}
      <div className="relative shrink-0">
        <div className="hscroll flex gap-1.5 px-5 pb-4">
          {categories.map((c) => {
            const active = c.id === activeCatId
            return (
              <button
                key={c.id}
                onClick={() => setActiveCatId(c.id)}
                className={cn(
                  "press px-4 py-2.5 rounded-full border text-[13px] whitespace-nowrap transition-[background,color,border-color] duration-[var(--dur-fast)]",
                  active
                    ? "font-semibold border-[var(--accent)] bg-[var(--accent-soft)] text-[var(--accent)] shadow-[inset_0_0_0_1px_var(--accent),var(--shadow-1)]"
                    : "font-medium border-[var(--line-2)] bg-[var(--bg-elev-1)] text-[var(--ink-2)] shadow-[var(--shadow-1)]"
                )}
              >
                {c.name}
              </button>
            )
          })}
        </div>
        {/* Right fade mask */}
        <div style={{
          position: "absolute", right: 0, top: 0, bottom: 0, width: 32, pointerEvents: "none",
          background: "linear-gradient(to right, transparent, var(--bg-base))",
        }} />
      </div>

      {/* Item list */}
      <div className="scrollarea flex-1 overflow-y-auto" style={{ paddingBottom: itemCount > 0 ? 72 : 24 }}>
        {activeCategory && (
          <div style={{ padding: "0 20px 28px" }}>
            <div className="eyebrow" style={{ marginBottom: 14 }}>
              {activeCategory.name} · {activeCategory.items.length} {activeCategory.items.length === 1 ? "dish" : "dishes"}
            </div>
            <div style={{ display: "flex", flexDirection: "column" }}>
              {activeCategory.items.map((item, idx, arr) => {
                const qty = cartCountByItem[item.id] ?? 0
                return (
                  <div key={item.id} className="presence">
                    <button
                      onClick={() => item.is_available && openSheet(item)}
                      disabled={!item.is_available}
                      className="press"
                      style={{
                        border: 0, background: "transparent", padding: "18px 0",
                        display: "flex", gap: 16, alignItems: "flex-start", textAlign: "left",
                        width: "100%", opacity: item.is_available ? 1 : 0.4,
                      }}
                      aria-disabled={!item.is_available}
                    >
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                          <span className="serif" style={{ fontSize: 20, fontWeight: 500, letterSpacing: "-0.015em", color: "var(--ink-1)", lineHeight: 1.15 }}>
                            {item.name}
                          </span>
                          <span className="leader" />
                          <span className="serif" style={{ fontSize: 17, color: "var(--accent)", fontWeight: 500, whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>
                            {formatCurrency(item.price)}
                          </span>
                        </div>
                        {item.description && (
                          <div style={{ color: "var(--ink-3)", fontSize: 13, lineHeight: 1.6, marginTop: 5 }}>
                            {item.description}
                          </div>
                        )}
                        {!item.is_available && (
                          <div style={{ color: "var(--ink-4)", fontSize: 11, marginTop: 5, letterSpacing: "0.05em", textTransform: "uppercase" }}>
                            Not available
                          </div>
                        )}
                      </div>
                      {/* Vignette + qty dot */}
                      <div style={{ position: "relative", flexShrink: 0 }}>
                        <Vignette hue={itemHue(item.id)} size={66} ring={qty > 0} />
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
          padding: "10px 16px 8px", flexShrink: 0,
          background: "color-mix(in srgb, var(--bg-base) 85%, transparent)",
          backdropFilter: "blur(20px)", WebkitBackdropFilter: "blur(20px)",
          borderTop: "1px solid var(--line-1)",
          animation: "slideUp 0.32s var(--ease-out)",
        }}>
          <button
            onClick={() => router.push(`/session/${sessionId}/cart`)}
            className="press btn-primary"
            style={{
              width: "100%", height: 54, borderRadius: 16,
              display: "flex", alignItems: "center", justifyContent: "space-between",
              padding: "0 20px",
            }}
          >
            <span style={{ display: "inline-flex", alignItems: "center", gap: 10, fontSize: 14, fontWeight: 600 }}>
              <span style={{
                width: 24, height: 24, borderRadius: 999,
                background: "rgba(0,0,0,0.2)", color: "inherit",
                display: "inline-flex", alignItems: "center", justifyContent: "center",
                fontSize: 12, fontWeight: 700,
              }}>{itemCount}</span>
              View cart
            </span>
            <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 14, fontWeight: 600 }}>
              {cartTotal > 0 ? formatCurrency(cartTotal) : ""} <ChevronRight style={{ width: 14, height: 14 }} aria-hidden />
            </span>
          </button>
        </div>
      )}

      {/* Item bottom sheet — cinematic */}
      <BottomSheet
        open={!!sheet}
        onClose={() => setSheet(null)}
        title={sheet?.item.name}
      >
        {sheet && (
          <div style={{ display: "flex", flexDirection: "column", gap: 22, paddingBottom: 8 }}>
            {/* Atmospheric accent strip */}
            <div style={{
              height: 3, borderRadius: 2,
              background: "linear-gradient(90deg, var(--accent), var(--accent-soft) 70%, transparent)",
              marginTop: -4,
            }} />

            {/* Description */}
            {sheet.item.description && (
              <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.65 }}>
                {sheet.item.description}
              </p>
            )}

            {/* Price + stepper */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <span className="serif" style={{ fontSize: 30, fontWeight: 500, color: "var(--accent)", letterSpacing: "-0.015em" }}>
                {formatCurrency(sheet.item.price)}
              </span>
              <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: Math.max(1, s.quantity - 1) } : s)}
                  style={{
                    width: 40, height: 40, borderRadius: 999,
                    border: "1px solid var(--line-2)", color: "var(--ink-1)",
                    background: "var(--bg-elev-2)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    transition: "background var(--dur-fast) var(--ease)",
                  }}
                  aria-label="Decrease quantity"
                >
                  <Minus style={{ width: 14, height: 14 }} aria-hidden />
                </button>
                <span style={{ width: 28, textAlign: "center", fontWeight: 600, fontSize: 17, color: "var(--ink-1)" }}>
                  {sheet.quantity}
                </span>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: s.quantity + 1 } : s)}
                  style={{
                    width: 40, height: 40, borderRadius: 999,
                    background: "var(--accent)", color: "var(--accent-ink)",
                    border: "none",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    transition: "opacity var(--dur-fast) var(--ease)",
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
                <div style={{ borderRadius: "var(--rad-lg)", overflow: "hidden", border: "1px solid var(--line-2)" }}>
                  {sheet.item.modifiers.map((mod: ItemModifier, i: number) => (
                    <label
                      key={mod.id}
                      style={{
                        display: "flex", alignItems: "center", justifyContent: "space-between",
                        padding: "14px 16px", cursor: "default",
                        borderTop: i > 0 ? "1px solid var(--line-1)" : "none",
                        background: sheet.selectedModifiers.includes(mod.id) ? "var(--accent-soft)" : "transparent",
                        transition: "background var(--dur-fast) var(--ease)",
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
                  width: "100%", borderRadius: "var(--rad-lg)", padding: "11px 14px",
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
                width: "100%", height: 54, borderRadius: 16,
                background: adding
                  ? "var(--bg-elev-3)"
                  : "linear-gradient(180deg, var(--accent-strong), var(--accent))",
                color: adding ? "var(--ink-3)" : "var(--accent-ink)",
                fontSize: 16, fontWeight: 600,
                border: adding ? "1px solid var(--line-2)" : "1px solid var(--accent)",
                boxShadow: adding ? "none" : "var(--shadow-2), inset 0 1px 0 rgba(255,255,255,0.18)",
                transition: "background var(--dur-fast) var(--ease)",
              }}
            >
              {adding
                ? "Adding…"
                : `Add to order · ${formatCurrency(
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
