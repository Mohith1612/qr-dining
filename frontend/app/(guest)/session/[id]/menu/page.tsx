"use client"

import { useEffect, useState } from "react"
import { useMenuStore } from "@/store/menu"
import { useCartStore } from "@/store/cart"
import { menuApi } from "@/lib/api/menu"
import { useCart } from "@/hooks/useCart"
import { useSession } from "@/hooks/useSession"
import { formatCurrency } from "@/lib/format"
import { MenuSkeleton } from "@/components/shared/LoadingSkeleton"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Plus, Minus, ShoppingCart } from "lucide-react"
import { toast } from "sonner"
import { useRouter } from "next/navigation"
import { use } from "react"
import type { MenuItem, ItemModifier } from "@/types/api"

interface ItemSheetState {
  item: MenuItem
  quantity: number
  selectedModifiers: number[]
  note: string
}

interface Props {
  params: Promise<{ id: string }>
}

export default function MenuPage({ params }: Props) {
  const { id: sessionId } = use(params)
  const router = useRouter()
  const categories = useMenuStore((s) => s.categories)
  const menuLoading = useMenuStore((s) => s.loading)
  const { session } = useSession()
  const { items: cartItems, itemCount, addItem } = useCart()
  const [sheet, setSheet] = useState<ItemSheetState | null>(null)
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

  function openSheet(item: MenuItem) {
    setSheet({ item, quantity: 1, selectedModifiers: [], note: "" })
  }

  function toggleModifier(id: number) {
    if (!sheet) return
    setSheet((s) =>
      s
        ? {
            ...s,
            selectedModifiers: s.selectedModifiers.includes(id)
              ? s.selectedModifiers.filter((m) => m !== id)
              : [...s.selectedModifiers, id],
          }
        : s
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

  // Count how many of each item ID are in the cart
  const cartCountByItem: Record<number, number> = {}
  for (const ci of cartItems) {
    cartCountByItem[ci.menu_item_id] = (cartCountByItem[ci.menu_item_id] ?? 0) + ci.quantity
  }

  if (menuLoading) return <MenuSkeleton />

  const firstCategory = categories[0]?.name ?? "menu"

  return (
    <div className="flex flex-col h-full">
      <Tabs defaultValue={firstCategory} className="flex-1 flex flex-col">
        {/* Category tab bar */}
        <div
          className="sticky top-0 z-20 border-b"
          style={{ backgroundColor: "var(--color-bg)", borderColor: "var(--color-border)" }}
        >
          <div className="overflow-x-auto scrollbar-none px-5">
            <TabsList className="h-auto gap-0 bg-transparent p-0 pb-0 flex-nowrap w-max">
              {categories.map((cat) => (
                <TabsTrigger
                  key={cat.id}
                  value={cat.name}
                  className="relative px-4 py-3.5 text-sm font-medium whitespace-nowrap bg-transparent rounded-none border-0 data-[state=active]:bg-transparent data-[state=active]:shadow-none transition-colors"
                  style={{
                    color: "var(--color-text-muted)",
                  }}
                >
                  <span className="relative">
                    {cat.name}
                  </span>
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto pb-28">
          {categories.map((cat) => (
            <TabsContent key={cat.id} value={cat.name} className="mt-0">
              <div className="divide-y" style={{ borderColor: "var(--color-border)" }}>
                {cat.items.map((item) => {
                  const inCartCount = cartCountByItem[item.id] ?? 0
                  return (
                    <button
                      key={item.id}
                      onClick={() => item.is_available && openSheet(item)}
                      disabled={!item.is_available}
                      className="w-full flex gap-4 items-center text-left px-5 py-4 transition-opacity active:opacity-60"
                      style={{ opacity: item.is_available ? 1 : 0.4 }}
                      aria-disabled={!item.is_available}
                    >
                      <div className="flex-1 space-y-1 min-w-0">
                        <div className="flex items-start gap-2">
                          <p
                            className="font-medium leading-snug text-base"
                            style={{
                              fontFamily: "var(--font-display)",
                              color: "var(--color-text)",
                              fontSize: "16px",
                            }}
                          >
                            {item.name}
                          </p>
                          {inCartCount > 0 && (
                            <span
                              className="flex-shrink-0 text-xs font-semibold px-1.5 py-0.5 rounded-full mt-0.5"
                              style={{
                                backgroundColor: "var(--color-accent)",
                                color: "var(--color-accent-fg)",
                                fontSize: "10px",
                              }}
                            >
                              {inCartCount}
                            </span>
                          )}
                        </div>
                        {item.description && (
                          <p className="text-xs line-clamp-2 leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
                            {item.description}
                          </p>
                        )}
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-semibold" style={{ color: "var(--color-accent)" }}>
                            {formatCurrency(item.price)}
                          </p>
                          {!item.is_available && (
                            <span className="text-xs" style={{ color: "var(--color-text-muted)" }}>
                              · Not available
                            </span>
                          )}
                        </div>
                      </div>
                      {item.is_available && (
                        <div
                          className="size-9 rounded-full flex items-center justify-center flex-shrink-0"
                          style={{
                            backgroundColor: inCartCount > 0 ? "var(--color-accent)" : "var(--color-surface)",
                            border: inCartCount > 0 ? "none" : "1px solid var(--color-border)",
                            color: inCartCount > 0 ? "var(--color-accent-fg)" : "var(--color-text-muted)",
                          }}
                          aria-hidden
                        >
                          <Plus className="size-4" />
                        </div>
                      )}
                    </button>
                  )
                })}
              </div>
            </TabsContent>
          ))}
        </div>
      </Tabs>

      {/* Cart bar */}
      {itemCount > 0 && (
        <div
          className="fixed bottom-[3.25rem] left-0 right-0 px-5 z-30"
          style={{ paddingBottom: "calc(0.625rem + env(safe-area-inset-bottom))", paddingTop: "0.5rem" }}
        >
          <button
            onClick={() => router.push(`/session/${sessionId}/cart`)}
            className="w-full h-13 rounded-2xl flex items-center justify-between px-5 font-medium transition-opacity active:opacity-80"
            style={{
              backgroundColor: "var(--color-accent)",
              color: "var(--color-accent-fg)",
              height: "52px",
              boxShadow: "var(--shadow-elevated)",
            }}
          >
            <span className="flex items-center gap-2.5">
              <ShoppingCart className="size-4" aria-hidden />
              <span className="text-sm font-semibold">{itemCount} {itemCount === 1 ? "item" : "items"}</span>
            </span>
            <span className="text-sm font-semibold">View Cart →</span>
          </button>
        </div>
      )}

      {/* Item detail bottom sheet */}
      <BottomSheet
        open={!!sheet}
        onClose={() => setSheet(null)}
        title={sheet?.item.name}
      >
        {sheet && (
          <div className="space-y-6 pb-2">
            {sheet.item.description && (
              <p className="text-sm leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
                {sheet.item.description}
              </p>
            )}

            {/* Price + quantity */}
            <div className="flex items-center justify-between">
              <span
                className="text-2xl font-medium"
                style={{ fontFamily: "var(--font-display)", color: "var(--color-accent)" }}
              >
                {formatCurrency(sheet.item.price)}
              </span>
              <div className="flex items-center gap-3">
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: Math.max(1, s.quantity - 1) } : s)}
                  className="size-9 rounded-full flex items-center justify-center border transition-opacity active:opacity-60"
                  style={{ borderColor: "var(--color-border)", color: "var(--color-text)" }}
                  aria-label="Decrease quantity"
                >
                  <Minus className="size-3.5" aria-hidden />
                </button>
                <span className="w-7 text-center font-semibold text-base">{sheet.quantity}</span>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: s.quantity + 1 } : s)}
                  className="size-9 rounded-full flex items-center justify-center transition-opacity active:opacity-60"
                  style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
                  aria-label="Increase quantity"
                >
                  <Plus className="size-3.5" aria-hidden />
                </button>
              </div>
            </div>

            {/* Modifiers */}
            {sheet.item.modifiers && sheet.item.modifiers.length > 0 && (
              <div className="space-y-3">
                <p
                  className="text-xs font-semibold uppercase tracking-widest"
                  style={{ color: "var(--color-text-muted)", letterSpacing: "0.1em" }}
                >
                  Customise
                </p>
                <div
                  className="rounded-xl overflow-hidden"
                  style={{ border: "1px solid var(--color-border)" }}
                >
                  {sheet.item.modifiers.map((mod: ItemModifier, i: number) => (
                    <label
                      key={mod.id}
                      className="flex items-center justify-between px-4 py-3.5 cursor-pointer transition-opacity active:opacity-70"
                      style={{
                        borderTop: i > 0 ? "1px solid var(--color-border)" : undefined,
                      }}
                    >
                      <span className="text-sm" style={{ color: "var(--color-text)" }}>{mod.name}</span>
                      <div className="flex items-center gap-3">
                        {mod.price_delta !== 0 && (
                          <span className="text-xs font-medium" style={{ color: "var(--color-text-muted)" }}>
                            +{formatCurrency(mod.price_delta)}
                          </span>
                        )}
                        <input
                          type="checkbox"
                          checked={sheet.selectedModifiers.includes(mod.id)}
                          onChange={() => toggleModifier(mod.id)}
                          className="size-5 rounded"
                          style={{ accentColor: "var(--color-accent)" }}
                        />
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* Note */}
            <div className="space-y-2">
              <p
                className="text-xs font-semibold uppercase tracking-widest"
                style={{ color: "var(--color-text-muted)", letterSpacing: "0.1em" }}
              >
                Special note
              </p>
              <Input
                value={sheet.note}
                onChange={(e) => setSheet((s) => s ? { ...s, note: e.target.value } : s)}
                placeholder="e.g. No onions, extra spicy…"
                className="text-sm rounded-xl"
                style={{
                  backgroundColor: "var(--color-surface-inset)",
                  borderColor: "var(--color-border)",
                  color: "var(--color-text)",
                }}
              />
            </div>

            {/* Add to cart CTA */}
            <Button
              onClick={handleAddToCart}
              disabled={adding}
              className="w-full rounded-xl font-medium text-base"
              style={{
                backgroundColor: "var(--color-accent)",
                color: "var(--color-accent-fg)",
                height: "52px",
              }}
            >
              {adding
                ? "Adding…"
                : `Add to cart — ${formatCurrency(
                    (parseFloat(sheet.item.price) +
                      sheet.selectedModifiers.reduce((sum, id) => {
                        const mod = sheet.item.modifiers?.find((m) => m.id === id)
                        return sum + (mod ? parseFloat(String(mod.price_delta)) : 0)
                      }, 0)) * sheet.quantity
                  )}`
              }
            </Button>
          </div>
        )}
      </BottomSheet>
    </div>
  )
}
