"use client"

import { use, useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useCart } from "@/hooks/useCart"
import { useOrders } from "@/hooks/useOrders"
import { useCartStore } from "@/store/cart"
import { formatCurrency } from "@/lib/format"
import { CartSkeleton } from "@/components/shared/LoadingSkeleton"
import { EmptyState } from "@/components/shared/EmptyState"
import { Vignette } from "@/components/shared/Vignette"
import { Trash2, ShoppingCart } from "lucide-react"
import { toast } from "sonner"
import type { CartItem } from "@/types/api"

interface Props {
  params: Promise<{ id: string }>
}

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

function CartItemRow({ item, onRemove }: { item: CartItem; onRemove: (id: number) => void }) {
  const modifierTotal = item.selected_modifiers?.reduce((sum, m) => sum + m.price_delta, 0) ?? 0
  const unitPrice = (item.item_price ?? 0) + modifierTotal

  return (
    <div style={{ display: "flex", gap: 14, alignItems: "flex-start", padding: "18px 0", borderBottom: "1px solid var(--line-1)" }}>
      {/* Vignette + quantity */}
      <div style={{ position: "relative", flexShrink: 0 }}>
        <Vignette hue={itemHue(item.menu_item_id)} size={56} />
        <span style={{
          position: "absolute", top: -4, right: -4,
          width: 20, height: 20, borderRadius: "50%",
          background: "var(--accent)", border: "2px solid var(--bg-base)",
          display: "flex", alignItems: "center", justifyContent: "center",
          fontSize: 10, fontWeight: 700, color: "var(--accent-ink)",
        }}>
          {item.quantity}
        </span>
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        <p className="serif" style={{ fontSize: 17, fontWeight: 500, color: "var(--ink-1)", lineHeight: 1.2, marginBottom: 3 }}>
          {item.item_name ?? `Item #${item.menu_item_id}`}
        </p>
        {item.selected_modifiers?.length > 0 && (
          <p style={{ fontSize: 12, color: "var(--ink-3)", lineHeight: 1.5 }}>
            {item.selected_modifiers.map((m) => m.name).join(" · ")}
          </p>
        )}
        {item.note && (
          <p style={{ fontSize: 12, fontStyle: "italic", color: "var(--ink-3)", marginTop: 2 }}>"{item.note}"</p>
        )}
      </div>

      <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 8, flexShrink: 0 }}>
        {unitPrice > 0 && (
          <p className="serif" style={{ fontSize: 16, fontWeight: 500, color: "var(--accent)" }}>
            {formatCurrency(unitPrice * item.quantity)}
          </p>
        )}
        <button
          onClick={() => onRemove(item.id)}
          style={{
            width: 30, height: 30, borderRadius: "50%",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
            color: "var(--ink-4)", display: "flex", alignItems: "center", justifyContent: "center",
            cursor: "pointer",
            transition: "color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease)",
          }}
          aria-label="Remove item"
        >
          <Trash2 size={13} aria-hidden />
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
      <div style={{ minHeight: "70vh", display: "flex", flexDirection: "column" }}>
        <EmptyState
          icon={ShoppingCart}
          eyebrow="Your selection"
          title="Nothing here yet"
          description="Browse the menu to add items to your order."
          action={{ label: "Browse menu", onClick: () => router.push(`/session/${sessionId}/menu`) }}
        />
      </div>
    )
  }

  const itemCount = items.reduce((s, i) => s + i.quantity, 0)
  const subtotal = items.reduce((s, i) => {
    const modTotal = i.selected_modifiers?.reduce((sum, m) => sum + m.price_delta, 0) ?? 0
    return s + ((i.item_price ?? 0) + modTotal) * i.quantity
  }, 0)

  return (
    <div className="screen-enter" style={{ padding: "24px 20px 32px", background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <p className="eyebrow">Your selection</p>
        <h1 className="display-lg" style={{ margin: "6px 0 0" }}>
          Ready to send
        </h1>
      </div>

      {/* Item list */}
      <div style={{ marginBottom: 24 }}>
        {items.map((item) => (
          <CartItemRow key={item.id} item={item} onRemove={handleRemove} />
        ))}
      </div>

      {/* Totals card */}
      <div
        className="atmos"
        style={{
          background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
          borderRadius: "var(--rad-lg)", boxShadow: "var(--shadow-2)",
          padding: "18px 20px", marginBottom: 24,
        }}
      >
        <p className="eyebrow" style={{ marginBottom: 12 }}>
          {itemCount} {itemCount === 1 ? "item" : "items"}
        </p>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <span className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--accent)", letterSpacing: "-0.015em" }}>
            {formatCurrency(subtotal)}
          </span>
          <span style={{ fontSize: 11, color: "var(--ink-4)", fontStyle: "italic" }}>Confirmed at table</span>
        </div>
      </div>

      {/* CTA */}
      <button
        onClick={handlePlaceOrder}
        disabled={placing || items.length === 0}
        className="press"
        style={{
          width: "100%", height: 54, borderRadius: 16,
          background: placing
            ? "var(--bg-elev-3)"
            : "linear-gradient(180deg, var(--accent-strong), var(--accent))",
          color: placing ? "var(--ink-3)" : "var(--accent-ink)",
          border: placing ? "1px solid var(--line-2)" : "1px solid var(--accent)",
          boxShadow: placing ? "none" : "var(--shadow-2), inset 0 1px 0 rgba(255,255,255,0.18)",
          fontSize: 16, fontWeight: 600,
          cursor: placing ? "not-allowed" : "pointer",
          transition: "background var(--dur-fast) var(--ease)",
        }}
      >
        {placing ? "Sending to kitchen…" : `Confirm your order · ${formatCurrency(subtotal)}`}
      </button>
    </div>
  )
}
