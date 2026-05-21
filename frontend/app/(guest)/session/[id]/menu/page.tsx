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
      toast.success(`${sheet.item.name} added to cart`)
      setSheet(null)
    } catch {
      toast.error("Couldn't add item. Please try again.")
    } finally {
      setAdding(false)
    }
  }

  if (menuLoading) return <MenuSkeleton />

  const firstCategory = categories[0]?.name ?? "menu"

  return (
    <div className="flex flex-col h-full">
      <Tabs defaultValue={firstCategory} className="flex-1 flex flex-col">
        <div
          className="sticky top-0 z-20 border-b px-4 pt-2"
          style={{ backgroundColor: "var(--color-bg)", borderColor: "var(--color-border)" }}
        >
          <div className="overflow-x-auto scrollbar-none">
            <TabsList className="h-auto gap-1 bg-transparent p-0 pb-2 flex-nowrap w-max">
              {categories.map((cat) => (
                <TabsTrigger
                  key={cat.id}
                  value={cat.name}
                  className="rounded-full px-4 py-1.5 text-sm font-medium whitespace-nowrap data-[state=active]:bg-accent data-[state=active]:text-accent-foreground"
                >
                  {cat.name}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto pb-24">
          {categories.map((cat) => (
            <TabsContent key={cat.id} value={cat.name} className="mt-0 px-4 py-4 space-y-3">
              {cat.items.map((item) => (
                <button
                  key={item.id}
                  onClick={() => item.is_available && openSheet(item)}
                  disabled={!item.is_available}
                  className="w-full flex gap-3 items-start text-left transition-opacity active:opacity-60"
                  style={{ opacity: item.is_available ? 1 : 0.45 }}
                  aria-disabled={!item.is_available}
                >
                  <div className="flex-1 space-y-1 py-1">
                    <div className="flex items-start justify-between gap-2">
                      <p className="font-medium text-sm leading-snug" style={{ color: "var(--color-text)" }}>
                        {item.name}
                        {!item.is_available && (
                          <span className="ml-2 text-xs font-normal" style={{ color: "var(--color-text-muted)" }}>
                            Not available
                          </span>
                        )}
                      </p>
                    </div>
                    {item.description && (
                      <p className="text-xs line-clamp-2" style={{ color: "var(--color-text-muted)" }}>
                        {item.description}
                      </p>
                    )}
                    <p className="text-sm font-semibold" style={{ color: "var(--color-accent)" }}>
                      {formatCurrency(item.price)}
                    </p>
                  </div>
                  {item.is_available && (
                    <div
                      className="size-8 rounded-full flex items-center justify-center flex-shrink-0 mt-1"
                      style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
                      aria-hidden
                    >
                      <Plus className="size-4" />
                    </div>
                  )}
                </button>
              ))}
            </TabsContent>
          ))}
        </div>
      </Tabs>

      {itemCount > 0 && (
        <div
          className="fixed bottom-16 left-0 right-0 px-4 pb-2 z-30"
          style={{ paddingBottom: "calc(0.5rem + env(safe-area-inset-bottom))" }}
        >
          <Button
            onClick={() => router.push(`/session/${sessionId}/cart`)}
            className="w-full h-12 rounded-xl flex items-center justify-between px-4 font-medium"
            style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
          >
            <span className="flex items-center gap-2">
              <ShoppingCart className="size-4" aria-hidden />
              {itemCount} {itemCount === 1 ? "item" : "items"}
            </span>
            <span>View Cart</span>
          </Button>
        </div>
      )}

      <BottomSheet
        open={!!sheet}
        onClose={() => setSheet(null)}
        title={sheet?.item.name}
      >
        {sheet && (
          <div className="space-y-5 pb-4">
            {sheet.item.description && (
              <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
                {sheet.item.description}
              </p>
            )}

            <div className="flex items-center justify-between">
              <span className="font-semibold text-lg" style={{ color: "var(--color-accent)" }}>
                {formatCurrency(sheet.item.price)}
              </span>
              <div className="flex items-center gap-3">
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: Math.max(1, s.quantity - 1) } : s)}
                  className="size-8 rounded-full flex items-center justify-center border"
                  style={{ borderColor: "var(--color-border)", color: "var(--color-text)" }}
                  aria-label="Decrease quantity"
                >
                  <Minus className="size-3.5" aria-hidden />
                </button>
                <span className="w-6 text-center font-medium text-sm">{sheet.quantity}</span>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: s.quantity + 1 } : s)}
                  className="size-8 rounded-full flex items-center justify-center"
                  style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
                  aria-label="Increase quantity"
                >
                  <Plus className="size-3.5" aria-hidden />
                </button>
              </div>
            </div>

            {sheet.item.modifiers && sheet.item.modifiers.length > 0 && (
              <div className="space-y-2">
                <p className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
                  Customise
                </p>
                {sheet.item.modifiers.map((mod: ItemModifier) => (
                  <label
                    key={mod.id}
                    className="flex items-center justify-between py-2 cursor-pointer"
                  >
                    <span className="text-sm" style={{ color: "var(--color-text)" }}>{mod.name}</span>
                    <div className="flex items-center gap-3">
                      {mod.price_delta !== 0 && (
                        <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
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
            )}

            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
                Special note
              </p>
              <Input
                value={sheet.note}
                onChange={(e) => setSheet((s) => s ? { ...s, note: e.target.value } : s)}
                placeholder="e.g. No onions, extra spicy…"
                className="text-sm"
              />
            </div>

            <Button
              onClick={handleAddToCart}
              disabled={adding}
              className="w-full h-12 rounded-xl font-medium text-base"
              style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
            >
              {adding ? "Adding…" : `Add to cart — ${formatCurrency(parseFloat(sheet.item.price) * sheet.quantity)}`}
            </Button>
          </div>
        )}
      </BottomSheet>
    </div>
  )
}
