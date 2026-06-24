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
import { Trash2, ShoppingCart, Loader2, CheckCircle, X } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import { promosApi } from "@/lib/api/promos"
import type { CartItem, ValidatePromoResponse } from "@/types/api"

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
          <p style={{ fontSize: 12, fontStyle: "italic", color: "var(--ink-3)", marginTop: 2 }}>&quot;{item.note}&quot;</p>
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
  const { session, participant } = useSession()
  const [placing, setPlacing] = useState(false)

  const [promoCode, setPromoCode] = useState("")
  const [appliedPromo, setAppliedPromo] = useState<ValidatePromoResponse | null>(null)
  const [appliedPromoCode, setAppliedPromoCode] = useState<string>("")
  const [promoLoading, setPromoLoading] = useState(false)
  const [promoError, setPromoError] = useState<string | null>(null)

  useEffect(() => {
    refreshCart()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleRemove(itemId: number) {
    try {
      await removeItem(itemId)
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't remove item.")
    }
  }

  async function handleApplyPromo() {
    if (!promoCode.trim() || !session) return
    setPromoLoading(true)
    setPromoError(null)
    try {
      const code = promoCode.trim().toUpperCase()
      const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
      const result = await promosApi.validate(session.id, code, guestToken)
      setAppliedPromo(result)
      setAppliedPromoCode(code)
      setPromoCode("")
    } catch (err) {
      if (err instanceof ApiError) {
        const msgs: Record<string, string> = {
          PROMO_NOT_FOUND:    "This promo code isn't valid right now.",
          MIN_ORDER_NOT_MET:  "Your order total doesn't meet this promo's minimum.",
          PROMO_EXHAUSTED:    "This offer has been claimed by too many guests.",
          PROMO_ALREADY_USED: "You've already used this offer.",
        }
        setPromoError(msgs[err.code] ?? "This promo code couldn't be applied.")
      } else {
        setPromoError("Couldn't validate the promo code. Please try again.")
      }
    } finally {
      setPromoLoading(false)
    }
  }

  function handleRemovePromo() {
    setAppliedPromo(null)
    setAppliedPromoCode("")
    setPromoError(null)
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
        })),
        appliedPromo ? appliedPromoCode : undefined
      )
      useCartStore.getState().clear()
      toast.success("Order placed!")
      router.push(`/session/${sessionId}/orders`)
    } catch (err) {
      if (err instanceof ApiError) {
        const promoMsgs: Record<string, string> = {
          PROMO_NOT_FOUND:    "Your promo code is no longer valid.",
          MIN_ORDER_NOT_MET:  "Your order total doesn't meet the promo's minimum.",
          PROMO_EXHAUSTED:    "This promo has reached its limit.",
          PROMO_ALREADY_USED: "You've already used this promo.",
        }
        if (promoMsgs[err.code]) {
          setAppliedPromo(null)
          toast.error(promoMsgs[err.code])
        } else {
          toast.error(friendlyErrorMessage(err.code))
        }
      } else {
        toast.error("Couldn't place order. Please try again.")
      }
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

      {/* Promo code section */}
      <div style={{ marginBottom: 16 }}>
        {appliedPromo ? (
          <div style={{
            display: "flex", alignItems: "center", gap: 10,
            padding: "12px 14px", borderRadius: "var(--rad-md)",
            background: "var(--ok-soft)", border: "1px solid var(--ok)",
          }}>
            <CheckCircle style={{ width: 16, height: 16, color: "var(--ok)", flexShrink: 0 }} aria-hidden />
            <div style={{ flex: 1, minWidth: 0 }}>
              <p style={{ fontSize: 13, fontWeight: 600, color: "var(--ok)", lineHeight: 1.3 }}>
                {appliedPromoCode} applied — saving {formatCurrency(appliedPromo.discount_amount)}
              </p>
              {appliedPromo.description && (
                <p style={{ fontSize: 12, color: "var(--ok)", opacity: 0.8, lineHeight: 1.4, marginTop: 2 }}>
                  {appliedPromo.description}
                </p>
              )}
            </div>
            <button
              onClick={handleRemovePromo}
              style={{
                width: 24, height: 24, borderRadius: "50%",
                background: "transparent", border: "none",
                color: "var(--ok)", cursor: "pointer", flexShrink: 0,
                display: "flex", alignItems: "center", justifyContent: "center",
              }}
              aria-label="Remove promo code"
            >
              <X size={14} aria-hidden />
            </button>
          </div>
        ) : (
          <div>
            <div style={{ display: "flex", gap: 8 }}>
              <input
                type="text"
                value={promoCode}
                onChange={(e) => { setPromoCode(e.target.value.toUpperCase()); setPromoError(null) }}
                onKeyDown={(e) => e.key === "Enter" && handleApplyPromo()}
                placeholder="Promo code"
                style={{
                  flex: 1, height: 42, borderRadius: "var(--rad-md)",
                  background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
                  padding: "0 12px", fontSize: 13, color: "var(--ink-1)",
                  outline: "none",
                }}
                aria-label="Promo code"
              />
              <button
                onClick={handleApplyPromo}
                disabled={promoLoading || !promoCode.trim()}
                className="press"
                style={{
                  height: 42, padding: "0 16px", borderRadius: "var(--rad-md)",
                  background: "var(--accent)", color: "var(--accent-ink)",
                  border: "1px solid var(--accent)", fontSize: 13, fontWeight: 500,
                  opacity: promoLoading || !promoCode.trim() ? 0.5 : 1,
                  display: "flex", alignItems: "center", gap: 6,
                  cursor: promoLoading || !promoCode.trim() ? "not-allowed" : "pointer",
                }}
                aria-label="Apply promo code"
              >
                {promoLoading
                  ? <Loader2 size={14} className="animate-spin" aria-hidden />
                  : "Apply"
                }
              </button>
            </div>
            {promoError && (
              <p style={{ marginTop: 6, fontSize: 12, color: "var(--err, #e05252)", lineHeight: 1.4 }}>
                {promoError}
              </p>
            )}
          </div>
        )}
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
        {appliedPromo && (
          <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
            <span style={{ fontSize: 13, color: "var(--ink-3)" }}>Subtotal</span>
            <span style={{ fontSize: 13, color: "var(--ink-2)" }}>{formatCurrency(subtotal)}</span>
          </div>
        )}
        {appliedPromo && (
          <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
            <span style={{ fontSize: 13, color: "var(--ok)" }}>Discount ({appliedPromoCode})</span>
            <span style={{ fontSize: 13, color: "var(--ok)", fontWeight: 500 }}>−{formatCurrency(appliedPromo.discount_amount)}</span>
          </div>
        )}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <span className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--accent)", letterSpacing: "-0.015em" }}>
            {formatCurrency(appliedPromo ? subtotal - appliedPromo.discount_amount : subtotal)}
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
