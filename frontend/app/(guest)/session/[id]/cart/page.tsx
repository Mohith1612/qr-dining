"use client"

import { use, useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useCart } from "@/hooks/useCart"
import { useOrders } from "@/hooks/useOrders"
import { useCartStore } from "@/store/cart"
import { formatCurrency } from "@/lib/format"
import { CartSkeleton } from "@/components/shared/LoadingSkeleton"
import { EmptyState } from "@/components/shared/EmptyState"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Trash2, ShoppingCart } from "lucide-react"
import { toast } from "sonner"
import type { CartItem } from "@/types/api"

interface Props {
  params: Promise<{ id: string }>
}

function CartItemRow({ item, onRemove }: { item: CartItem; onRemove: (id: number) => void }) {
  const modifierTotal = item.selected_modifiers?.reduce((sum, m) => sum + m.price_delta, 0) ?? 0
  const unitPrice = (item.item_price ?? 0) + modifierTotal

  return (
    <div className="flex gap-4 items-start py-4 border-b" style={{ borderColor: "var(--color-border)" }}>
      {/* Quantity badge */}
      <div
        className="size-8 rounded-xl flex items-center justify-center flex-shrink-0 mt-0.5 text-sm font-semibold"
        style={{ backgroundColor: "var(--color-surface-inset)", color: "var(--color-text-muted)" }}
      >
        {item.quantity}
      </div>

      <div className="flex-1 space-y-1 min-w-0">
        <p
          className="font-medium text-base leading-snug"
          style={{ fontFamily: "var(--font-display)", color: "var(--color-text)", fontSize: "16px" }}
        >
          {item.item_name ?? `Item #${item.menu_item_id}`}
        </p>
        {item.selected_modifiers?.length > 0 && (
          <p className="text-xs leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
            {item.selected_modifiers.map((m) => m.name).join(" · ")}
          </p>
        )}
        {item.note && (
          <p className="text-xs italic" style={{ color: "var(--color-text-muted)" }}>
            "{item.note}"
          </p>
        )}
      </div>

      <div className="flex flex-col items-end gap-2 flex-shrink-0">
        {unitPrice > 0 && (
          <p className="text-sm font-semibold" style={{ color: "var(--color-text)" }}>
            {formatCurrency(unitPrice * item.quantity)}
          </p>
        )}
        <button
          onClick={() => onRemove(item.id)}
          className="p-2 min-h-[44px] min-w-[44px] flex items-center justify-center rounded-xl transition-opacity active:opacity-60"
          style={{ color: "var(--color-text-muted)" }}
          aria-label="Remove item"
        >
          <Trash2 className="size-4" aria-hidden />
        </button>
      </div>
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
      <div style={{ backgroundColor: "var(--color-bg)" }} className="min-h-[60vh]">
        <EmptyState
          icon={ShoppingCart}
          title="Your cart is empty"
          description="Browse the menu to add items to your order."
          action={{ label: "Browse menu", onClick: () => router.push(`/session/${sessionId}/menu`) }}
        />
      </div>
    )
  }

  return (
    <div
      className="px-5 py-7 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <h1
        className="text-3xl font-medium"
        style={{ fontFamily: "var(--font-display)", color: "var(--color-text)" }}
      >
        Your order
      </h1>

      <div>
        {items.map((item) => (
          <CartItemRow key={item.id} item={item} onRemove={handleRemove} />
        ))}
      </div>

      <HospitalityCard variant="elevated" style={{ padding: "1.25rem" }}>
        <div className="flex justify-between text-sm items-center">
          <span style={{ color: "var(--color-text-muted)" }}>
            {items.reduce((s, i) => s + i.quantity, 0)} {items.reduce((s, i) => s + i.quantity, 0) === 1 ? "item" : "items"}
          </span>
          <span className="text-xs font-medium px-2.5 py-1 rounded-full" style={{ backgroundColor: "var(--color-surface-inset)", color: "var(--color-text-muted)" }}>
            Confirm total at counter
          </span>
        </div>
      </HospitalityCard>

      <Button
        onClick={handlePlaceOrder}
        disabled={placing || items.length === 0}
        className="w-full rounded-xl font-medium"
        style={{
          backgroundColor: "var(--color-accent)",
          color: "var(--color-accent-fg)",
          height: "52px",
          fontSize: "15px",
        }}
      >
        {placing ? "Placing order…" : "Place order"}
      </Button>
    </div>
  )
}
