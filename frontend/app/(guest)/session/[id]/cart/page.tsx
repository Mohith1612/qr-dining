"use client"

import { use, useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useCart } from "@/hooks/useCart"
import { useOrders } from "@/hooks/useOrders"
import { useCartStore } from "@/store/cart"
import { useSession } from "@/hooks/useSession"
import { formatCurrency } from "@/lib/format"
import { CartSkeleton } from "@/components/shared/LoadingSkeleton"
import { EmptyState } from "@/components/shared/EmptyState"
import { Vignette } from "@/components/shared/Vignette"
import { Trash2, ShoppingCart, ChevronRight, Plus } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import type { CartItem } from "@/types/api"
import { track } from "@/lib/product-analytics/events"

interface Props {
  params: Promise<{ id: string }>
}

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

function CartItemRow({ item, addedBy, onRemove }: { item: CartItem; addedBy?: string; onRemove: (id: number) => void }) {
  const [imgError, setImgError] = useState(false)
  const modifierTotal = item.selected_modifiers?.reduce((sum, m) => sum + m.price_delta, 0) ?? 0
  const unitPrice = (item.item_price ?? 0) + modifierTotal

  return (
    <div style={{ display: "flex", gap: 14, alignItems: "flex-start", padding: "16px 0", borderBottom: "1px solid var(--line-1)" }}>
      {/* Thumbnail (photo when available, vignette otherwise) + quantity */}
      <div style={{ position: "relative", flexShrink: 0 }}>
        {item.image_url && !imgError ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.image_url}
            alt=""
            aria-hidden
            loading="lazy"
            onError={() => setImgError(true)}
            style={{ width: 56, height: 56, borderRadius: 12, objectFit: "cover", display: "block" }}
          />
        ) : (
          <Vignette hue={itemHue(item.menu_item_id)} size={56} />
        )}
        <span style={{
          position: "absolute", top: -4, right: -4,
          minWidth: 20, height: 20, padding: "0 6px", borderRadius: 999,
          background: "var(--accent)", border: "2px solid var(--bg-base)",
          display: "flex", alignItems: "center", justifyContent: "center",
          fontSize: 10.5, fontWeight: 700, color: "var(--accent-ink)", fontVariantNumeric: "tabular-nums",
        }}>
          {item.quantity}
        </span>
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        <p style={{ fontSize: 15.5, fontWeight: 600, color: "var(--ink-1)", lineHeight: 1.25, marginBottom: 3, letterSpacing: "-0.005em" }}>
          {item.item_name ?? `Item #${item.menu_item_id}`}
        </p>
        {item.selected_modifiers?.length > 0 && (
          <p style={{ fontSize: 12.5, color: "var(--ink-3)", lineHeight: 1.5 }}>
            {item.selected_modifiers.map((m) => m.name).join(" · ")}
          </p>
        )}
        {item.note && (
          <p data-ph-mask style={{ fontSize: 12.5, fontStyle: "italic", color: "var(--ink-3)", marginTop: 2 }}>&quot;{item.note}&quot;</p>
        )}
        {addedBy && (
          <p data-ph-mask style={{ fontSize: 11.5, color: "var(--ink-4)", marginTop: 4 }}>Added by {addedBy}</p>
        )}
      </div>

      <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 10, flexShrink: 0 }}>
        {unitPrice > 0 && (
          <p style={{ fontSize: 15, fontWeight: 600, color: "var(--ink-1)", fontVariantNumeric: "tabular-nums" }}>
            {formatCurrency(unitPrice * item.quantity)}
          </p>
        )}
        <button
          onClick={() => onRemove(item.id)}
          style={{
            width: 30, height: 30, borderRadius: "50%",
            background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
            color: "var(--ink-3)", display: "flex", alignItems: "center", justifyContent: "center",
            cursor: "pointer", transition: "color var(--dur-fast) var(--ease)",
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
  const { participant, participants, isHost } = useSession()
  const [placing, setPlacing] = useState(false)

  const hostName = participants.find((p) => p.is_host)?.display_name
  // When the host has left and nobody holds the role, let any remaining guest
  // send the order — the backend promotes whoever places it (AuthorizeHostAction),
  // so the table is never stranded unable to order.
  const hasHost = participants.some((p) => p.is_host)
  const canSend = isHost || !hasHost

  // Resolve who added each item for the shared-cart "Added by" line.
  function addedByName(pid: number): string | undefined {
    const p = participants.find((pp) => pp.id === pid)
    if (!p) return undefined
    return p.id === participant?.id ? "you" : p.display_name
  }

  // Promo codes are entered at the bill (payment screen), not on the cart.

  useEffect(() => {
    refreshCart()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleRemove(itemId: number) {
    try {
      const removed = items.find((item) => item.id === itemId)
      const succeeded = await removeItem(itemId)
      if (succeeded && removed) {
        track("cart_item_removed", { item_id: removed.menu_item_id, quantity: removed.quantity })
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't remove item.")
    }
  }

  async function handlePlaceOrder() {
    if (items.length === 0) return
    setPlacing(true)
    try {
      const result = await placeOrder(
        items.map((item) => ({
          menu_item_id: item.menu_item_id,
          quantity: item.quantity,
          modifier_ids: item.selected_modifiers?.map((m) => m.id),
          note: item.note || undefined,
        }))
      )
      if (!result) {
        setPlacing(false)
        return
      }
      const itemCount = items.reduce((sum, item) => sum + item.quantity, 0)
      const subtotal = items.reduce((sum, item) => {
        const modifiers = item.selected_modifiers.reduce((value, modifier) => value + modifier.price_delta, 0)
        return sum + ((item.item_price ?? 0) + modifiers) * item.quantity
      }, 0)
      track("order_placed", { order_id: result.order.id, item_count: itemCount, subtotal })
      useCartStore.getState().clear()
      toast.success("Order placed!")
      router.push(`/session/${sessionId}/orders`)
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't place order. Please try again.")
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
    <>
      <div className="screen-enter" style={{ padding: "24px 20px 104px", background: "var(--bg-base)", color: "var(--ink-1)" }}>
        {/* Header */}
        <div style={{ marginBottom: 20 }}>
          <p className="eyebrow">Your selection</p>
          <h1 className="display-lg" style={{ margin: "6px 0 0" }}>
            Review order
          </h1>
        </div>

        {/* Item list */}
        <div style={{ marginBottom: 18 }}>
          {items.map((item) => (
            <CartItemRow key={item.id} item={item} addedBy={addedByName(item.participant_id)} onRemove={handleRemove} />
          ))}
        </div>

        {/* Add more items */}
        <button
          onClick={() => router.push(`/session/${sessionId}/menu`)}
          className="press"
          style={{
            display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
            width: "100%", padding: "13px", marginBottom: 22,
            borderRadius: "var(--rad-md)", border: "1px dashed var(--line-3)",
            background: "transparent", color: "var(--ink-2)", fontSize: 13.5, fontWeight: 600, cursor: "pointer",
          }}
        >
          <Plus size={15} aria-hidden /> Add more items
        </button>

        {/* Totals card */}
        <div
          style={{
            background: "var(--bg-elev-1)", border: "1px solid var(--line-1)",
            borderRadius: "var(--rad-lg)", boxShadow: "var(--shadow-1)",
            padding: "16px 18px",
          }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
            <span style={{ fontSize: 13.5, color: "var(--ink-2)" }}>Subtotal · {itemCount} {itemCount === 1 ? "item" : "items"}</span>
            <span style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)", fontVariantNumeric: "tabular-nums" }}>
              {formatCurrency(subtotal)}
            </span>
          </div>
          <p style={{ fontSize: 11.5, color: "var(--ink-4)", margin: 0 }}>Taxes &amp; charges are calculated at the bill.</p>
        </div>
      </div>

      {/* Fixed action — host sends the order; others see who confirms */}
      <div style={{
        position: "fixed", left: 0, right: 0,
        bottom: "calc(84px + env(safe-area-inset-bottom))", zIndex: 20,
        padding: "12px 16px",
        background: "var(--bg-overlay)",
        backdropFilter: "blur(20px) saturate(140%)", WebkitBackdropFilter: "blur(20px) saturate(140%)",
        borderTop: "1px solid var(--line-2)",
      }}>
        {canSend ? (
          <>
            {!isHost && (
              <p style={{ textAlign: "center", fontSize: 12, color: "var(--ink-3)", margin: "0 0 8px" }}>
                The host has left — you&apos;ll become the host when you send this order.
              </p>
            )}
            <button
              onClick={handlePlaceOrder}
              disabled={placing || items.length === 0}
              className="press"
              style={{
                width: "100%", height: 54, borderRadius: "var(--rad-md)",
                display: "flex", alignItems: "center", justifyContent: "space-between", padding: "0 20px",
                background: placing ? "var(--bg-sunken)" : "var(--accent)",
                color: placing ? "var(--ink-3)" : "var(--accent-ink)",
                border: "none",
                boxShadow: placing ? "none" : "var(--shadow-2)",
                fontSize: 15.5, fontWeight: 600,
                cursor: placing ? "not-allowed" : "pointer",
                transition: "background var(--dur-fast) var(--ease)",
              }}
            >
              <span>{placing ? "Sending to kitchen…" : "Place order"}</span>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontVariantNumeric: "tabular-nums" }}>
                {formatCurrency(subtotal)} <ChevronRight size={16} aria-hidden />
              </span>
            </button>
          </>
        ) : (
          <div style={{ textAlign: "center", padding: "2px 4px 4px" }}>
            <p data-ph-mask style={{ fontSize: 13.5, fontWeight: 600, color: "var(--ink-1)", marginBottom: 2 }}>
              {hostName ? `${hostName} sends the order` : "The table host sends the order"}
            </p>
            <p style={{ fontSize: 12, color: "var(--ink-3)", lineHeight: 1.5 }}>
              Keep adding to the shared cart — only the host confirms to the kitchen.
            </p>
          </div>
        )}
      </div>
    </>
  )
}
