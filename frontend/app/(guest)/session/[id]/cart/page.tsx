"use client"

import { use, useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useCart } from "@/hooks/useCart"
import { useOrders } from "@/hooks/useOrders"
import { useCartStore } from "@/store/cart"
import { formatCurrency } from "@/lib/format"
import { CartSkeleton } from "@/components/shared/LoadingSkeleton"
import { Button } from "@/components/ui/button"
import { Trash2, ShoppingCart } from "lucide-react"
import { toast } from "sonner"
import type { CartItem } from "@/types/api"

interface Props {
  params: Promise<{ id: string }>
}

function CartItemRow({ item, onRemove }: { item: CartItem; onRemove: (id: number) => void }) {
  const modifierTotal = item.selected_modifiers?.reduce((sum, m) => sum + m.price_delta, 0) ?? 0
  const unitPrice = parseFloat("0") + modifierTotal

  return (
    <div className="flex gap-3 items-start py-3 border-b" style={{ borderColor: "var(--color-border)" }}>
      <div className="flex-1 space-y-1">
        <p className="font-medium text-sm" style={{ color: "var(--color-text)" }}>
          {item.quantity}× Item #{item.menu_item_id}
        </p>
        {item.selected_modifiers?.length > 0 && (
          <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
            {item.selected_modifiers.map((m) => m.name).join(", ")}
          </p>
        )}
        {item.note && (
          <p className="text-xs italic" style={{ color: "var(--color-text-muted)" }}>
            "{item.note}"
          </p>
        )}
      </div>
      <button
        onClick={() => onRemove(item.id)}
        className="p-2 min-h-[44px] min-w-[44px] flex items-center justify-center"
        style={{ color: "var(--color-text-muted)" }}
        aria-label="Remove item"
      >
        <Trash2 className="size-4" aria-hidden />
      </button>
    </div>
  )
}

export default function CartPage({ params }: Props) {
  const { id: sessionId } = use(params)
  const router = useRouter()
  const { items, loading, removeItem, refreshCart } = useCart()
  const { placeOrder } = useOrders()
  const [placing, setPlacing] = useState(false)

  useEffect(() => {
    refreshCart()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleRemove(itemId: number) {
    try {
      await removeItem(itemId)
    } catch {
      toast.error("Couldn't remove item.")
    }
  }

  async function handlePlaceOrder() {
    if (items.length === 0) return
    setPlacing(true)
    try {
      await placeOrder(
        items.map((item) => ({
          menu_item_id: item.menu_item_id,
          quantity: item.quantity,
          modifier_ids: item.selected_modifiers?.map((m) => m.id),
          note: item.note || undefined,
        }))
      )
      useCartStore.getState().clear()
      toast.success("Order placed!")
      router.push(`/session/${sessionId}/orders`)
    } catch {
      toast.error("Couldn't place order. Please try again.")
      setPlacing(false)
    }
  }

  if (loading) return <CartSkeleton />

  if (items.length === 0) {
    return (
      <div
        className="min-h-[60vh] flex flex-col items-center justify-center px-6 text-center gap-4"
        style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
      >
        <div
          className="size-14 rounded-full flex items-center justify-center"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
        >
          <ShoppingCart className="size-6" style={{ color: "var(--color-text-muted)" }} aria-hidden />
        </div>
        <div className="space-y-1">
          <p className="font-medium">Your cart is empty</p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Browse the menu to add items
          </p>
        </div>
        <Button
          onClick={() => router.push(`/session/${sessionId}/menu`)}
          variant="outline"
          className="mt-2"
        >
          Browse menu
        </Button>
      </div>
    )
  }

  return (
    <div className="px-4 py-6 space-y-6" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}>
      <h1 className="text-lg font-semibold">Your cart</h1>

      <div>
        {items.map((item) => (
          <CartItemRow key={item.id} item={item} onRemove={handleRemove} />
        ))}
      </div>

      <div
        className="rounded-2xl p-4 space-y-3"
        style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
      >
        <div className="flex justify-between text-sm">
          <span style={{ color: "var(--color-text-muted)" }}>Items</span>
          <span className="font-medium">{items.reduce((s, i) => s + i.quantity, 0)}</span>
        </div>
        <div className="pt-2 border-t flex justify-between" style={{ borderColor: "var(--color-border)" }}>
          <span className="font-semibold">Total</span>
          <span className="font-semibold" style={{ color: "var(--color-accent)" }}>
            Confirm at counter
          </span>
        </div>
      </div>

      <Button
        onClick={handlePlaceOrder}
        disabled={placing || items.length === 0}
        className="w-full h-12 rounded-xl font-medium text-base"
        style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
      >
        {placing ? "Placing order…" : "Place order"}
      </Button>
    </div>
  )
}
