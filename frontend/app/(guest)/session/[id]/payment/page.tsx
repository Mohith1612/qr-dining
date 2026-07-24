"use client"

import { useState, useEffect } from "react"
import { useOrdersStore } from "@/store/orders"
import { useSession } from "@/hooks/useSession"
import { useSessionStore } from "@/store/session"
import { paymentsApi } from "@/lib/api/payments"
import { generateIdempotencyKey } from "@/lib/idempotency"
import { formatCurrency } from "@/lib/format"
import { Banknote, CreditCard, Smartphone, CheckCircle, Clock, Loader2, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import { promosApi } from "@/lib/api/promos"
import { X } from "lucide-react"
import type { PaymentMethod, PaymentStatus, BillData, ValidatePromoResponse } from "@/types/api"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { CustomerOptIn } from "@/components/shared/CustomerOptIn"
import { BillBreakdown } from "@/components/shared/BillBreakdown"

const PAYMENT_OPTIONS: {
  method: PaymentMethod
  label: string
  description: string
  icon: React.ElementType
}[] = [
  { method: "cash",    label: "Cash",      description: "A host will collect at the table",    icon: Banknote   },
  { method: "card",    label: "Card",       description: "Bring the POS terminal to the table", icon: CreditCard },
  { method: "digital", label: "UPI",        description: "GPay, PhonePe, or any UPI app",       icon: Smartphone },
]

const METHOD_LABEL: Record<PaymentMethod, string> = {
  cash: "Cash",
  card: "Card",
  digital: "UPI",
  card_manual: "Card",
  upi: "UPI",
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "6px 0" }}>
      <span style={{ color: "var(--ink-3)", fontSize: 13 }}>{label}</span>
      <span style={{ color: "var(--ink-1)", fontSize: 13, fontWeight: 500 }}>{value}</span>
    </div>
  )
}

export default function PaymentPage() {
  const orders = useOrdersStore((s) => s.orders)
  const { session, participant, participants, isHost } = useSession()
  const hostName = participants.find((p) => p.is_host)?.display_name
  const completedPayment = useSessionStore((s) => s.completedPayment)
  const [bill, setBill] = useState<BillData | null>(null)
  const [billLoading, setBillLoading] = useState(true)
  const [billError, setBillError] = useState<string | null>(null)
  const [loading, setLoading] = useState<PaymentMethod | null>(null)
  // The payment we initiated, with its server-assigned status. Cash/card come
  // back as requires_staff_confirmation — NOT completed — so the UI must show
  // "awaiting confirmation" until a PAYMENT_COMPLETED event confirms it.
  const [submitted, setSubmitted] = useState<{ method: PaymentMethod; status: PaymentStatus } | null>(null)
  const [paidTotal, setPaidTotal] = useState(0)
  const [showOptIn, setShowOptIn] = useState(false)

  // Promo applied at the bill. Validated against the live bill total; the
  // discount is previewed here and re-applied server-side at payment.
  const [promoCode, setPromoCode] = useState("")
  const [appliedPromo, setAppliedPromo] = useState<ValidatePromoResponse | null>(null)
  const [appliedPromoCode, setAppliedPromoCode] = useState("")
  const [appliedPromoPhone, setAppliedPromoPhone] = useState<string | undefined>(undefined)
  const [promoLoading, setPromoLoading] = useState(false)
  const [promoError, setPromoError] = useState<string | null>(null)
  const [promoPhone, setPromoPhone] = useState("")
  const [phoneRequired, setPhoneRequired] = useState(false)

  // Authoritative completion: either the initiate response already said
  // "completed" (e.g. an instantly-settled flow) or a PAYMENT_COMPLETED event
  // arrived over the websocket and was stored on the session.
  const isComplete = submitted?.status === "completed" || completedPayment !== null

  // Fetch bill on mount and when orders change (new order placed triggers WS → store update).
  useEffect(() => {
    if (!session?.id) return
    const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
    setBillLoading(true)
    setBillError(null)
    paymentsApi
      .getBill(session.id, guestToken)
      .then(setBill)
      .catch(() => setBillError("Could not load bill — please ask your waiter."))
      .finally(() => setBillLoading(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session?.id, orders.length])

  useEffect(() => {
    if (!submitted) return
    const timer = setTimeout(() => setShowOptIn(true), 1500)
    return () => clearTimeout(timer)
  }, [submitted])

  const billTotal = bill?.total ?? 0
  const discount = appliedPromo?.discount_amount ?? 0
  const total = Math.max(0, billTotal - discount)

  async function handleApplyPromo() {
    if (!promoCode.trim() || !session) return
    setPromoLoading(true)
    setPromoError(null)
    try {
      const code = promoCode.trim().toUpperCase()
      const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
      const phone = promoPhone.trim() ? "+91" + promoPhone.trim() : undefined
      const result = await promosApi.validate(session.id, code, billTotal, phone, guestToken)
      setAppliedPromo(result)
      setAppliedPromoCode(code)
      setAppliedPromoPhone(phone)
      setPromoCode("")
      setPhoneRequired(false)
    } catch (err) {
      if (err instanceof ApiError && err.code === "PROMO_PHONE_REQUIRED") {
        setPhoneRequired(true)
        setPromoError("Add your phone number to use this offer.")
      } else if (err instanceof ApiError) {
        const msgs: Record<string, string> = {
          PROMO_NOT_FOUND:    "This promo code isn't valid right now.",
          MIN_ORDER_NOT_MET:  "Your bill doesn't meet this promo's minimum.",
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
    setAppliedPromoPhone(undefined)
    setPhoneRequired(false)
    setPromoPhone("")
    setPromoError(null)
  }

  async function handlePay(method: PaymentMethod) {
    if (!session || submitted) return
    const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
    setLoading(method)
    try {
      // Fresh idempotency key per attempt, so editing the promo and retrying
      // is never blocked by the previous attempt's key.
      const promo = appliedPromo ? { code: appliedPromoCode, phoneE164: appliedPromoPhone } : undefined
      const payment = await paymentsApi.initiate(session.id, total, method, generateIdempotencyKey(), undefined, guestToken, promo)
      setPaidTotal(total)
      setSubmitted({ method, status: payment.status })
      toast.success(
        payment.status === "completed"
          ? "Payment completed."
          : "Request sent — your server will confirm."
      )
    } catch (err) {
      if (err instanceof ApiError) {
        const promoMsgs: Record<string, string> = {
          PROMO_NOT_FOUND:    "Your promo code is no longer valid.",
          MIN_ORDER_NOT_MET:  "Your bill doesn't meet the promo's minimum.",
          PROMO_EXHAUSTED:    "This promo has reached its limit.",
          PROMO_ALREADY_USED: "You've already used this promo.",
        }
        if (promoMsgs[err.code]) {
          handleRemovePromo()
          toast.error(promoMsgs[err.code])
        } else {
          toast.error(friendlyErrorMessage(err.code))
        }
      } else {
        toast.error("Couldn't process payment. Please try again.")
      }
    } finally {
      setLoading(null)
    }
  }

  if (submitted) {
    const tone = isComplete ? "var(--ok)" : "var(--info)"
    const toneSoft = isComplete ? "var(--ok-soft)" : "var(--info-soft)"
    return (
      <>
      <div
        className="flex-1 flex flex-col items-center justify-center px-7 text-center screen-enter"
        style={{ background: "var(--bg-base)" }}
      >
        <div style={{
          width: 84, height: 84, borderRadius: 999,
          background: toneSoft, border: `1px solid ${tone}`,
          display: "flex", alignItems: "center", justifyContent: "center",
          boxShadow: `0 0 0 8px ${toneSoft}, 0 12px 30px -10px rgba(0,0,0,0.4)`,
          marginBottom: 18,
        }}>
          {isComplete
            ? <CheckCircle style={{ width: 36, height: 36, color: tone }} aria-hidden />
            : <Clock style={{ width: 36, height: 36, color: tone }} aria-hidden />}
        </div>

        <p className="eyebrow" style={{ marginBottom: 6 }}>
          {isComplete ? "Payment confirmed" : "Awaiting confirmation"}
        </p>
        <h2 className="serif" style={{ margin: 0, fontSize: 32, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.02em" }}>
          {isComplete ? "Thank you" : "Almost there"}
        </h2>
        <p style={{ margin: "10px 0 22px", color: "var(--ink-2)", fontSize: 14, lineHeight: 1.6, maxWidth: 300 }}>
          {isComplete
            ? `Your payment of ${formatCurrency(paidTotal)} is confirmed.`
            : submitted.method === "cash"
            ? `Your server is on the way to collect ${formatCurrency(paidTotal)}. It'll show as paid once they confirm.`
            : submitted.method === "card"
            ? `Your server is bringing a card terminal for ${formatCurrency(paidTotal)}. It'll show as paid once they confirm.`
            : `We're confirming your ${formatCurrency(paidTotal)} UPI transfer.`}
        </p>

        <HospitalityCard elev={1} style={{ padding: "12px 16px", marginBottom: 22, minWidth: 240, textAlign: "left" }}>
          <Row label="Method" value={METHOD_LABEL[submitted.method]} />
          <Row label="Amount" value={paidTotal > 0 ? formatCurrency(paidTotal) : "—"} />
          <Row label="Status" value={isComplete ? "Paid" : "Awaiting confirmation"} />
          {session?.table_identifier && <Row label="Table" value={session.table_identifier} />}
        </HospitalityCard>

        <button
          onClick={() => window.history.back()}
          className="press"
          style={{
            padding: "10px 22px", borderRadius: "var(--rad-pill)",
            border: "1px solid var(--line-2)", color: "var(--ink-1)",
            background: "var(--bg-elev-1)", fontSize: 14, fontWeight: 500,
          }}
        >
          Return to table
        </button>
      </div>

      {showOptIn && session && (
        <CustomerOptIn
          sessionId={session.id}
          onComplete={() => setShowOptIn(false)}
        />
      )}
      </>
    )
  }

  return (
    <div className="scrollarea flex-1 overflow-y-auto screen-enter" style={{ background: "var(--bg-base)" }}>
      {/* Header */}
      <div className="page-glow" style={{ padding: "24px 20px 16px" }}>
        <span className="eyebrow">Your bill</span>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          Itemized bill
        </h1>
        <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 13 }}>
          Review your order before settling up.
        </p>
      </div>

      {/* Bill breakdown */}
      <div style={{ padding: "0 20px 8px" }}>
        <HospitalityCard elev={1} style={{ padding: "16px 18px" }}>
          <BillBreakdown bill={bill} loading={billLoading} error={billError} />
        </HospitalityCard>
      </div>

      {/* Promo code — host only, applied to the bill */}
      {isHost && billTotal > 0 && (
        <div style={{ padding: "8px 20px 0" }}>
          {appliedPromo ? (
            <div style={{
              display: "flex", alignItems: "center", gap: 10,
              padding: "12px 14px", borderRadius: "var(--rad-md)",
              background: "var(--ok-soft)", border: "1px solid var(--ok)",
            }}>
              <CheckCircle style={{ width: 16, height: 16, color: "var(--ok)", flexShrink: 0 }} aria-hidden />
              <div style={{ flex: 1, minWidth: 0 }}>
                <p style={{ fontSize: 13, fontWeight: 600, color: "var(--ok)", lineHeight: 1.3 }}>
                  {appliedPromoCode} applied — saving {formatCurrency(discount)}
                </p>
                {appliedPromo.description && (
                  <p style={{ fontSize: 12, color: "var(--ok)", opacity: 0.8, marginTop: 2 }}>{appliedPromo.description}</p>
                )}
              </div>
              <button onClick={handleRemovePromo} aria-label="Remove promo code" style={{
                width: 24, height: 24, borderRadius: "50%", background: "transparent",
                border: "none", color: "var(--ok)", cursor: "pointer", flexShrink: 0,
                display: "flex", alignItems: "center", justifyContent: "center",
              }}>
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
                  aria-label="Promo code"
                  style={{
                    flex: 1, height: 42, borderRadius: "var(--rad-md)",
                    background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
                    padding: "0 12px", fontSize: 13, color: "var(--ink-1)", outline: "none",
                  }}
                />
                <button
                  onClick={handleApplyPromo}
                  disabled={promoLoading || !promoCode.trim()}
                  className="press"
                  aria-label="Apply promo code"
                  style={{
                    height: 42, padding: "0 16px", borderRadius: "var(--rad-md)",
                    background: "var(--accent)", color: "var(--accent-ink)",
                    border: "1px solid var(--accent)", fontSize: 13, fontWeight: 500,
                    opacity: promoLoading || !promoCode.trim() ? 0.5 : 1,
                    display: "flex", alignItems: "center", gap: 6,
                    cursor: promoLoading || !promoCode.trim() ? "not-allowed" : "pointer",
                  }}
                >
                  {promoLoading ? <Loader2 size={14} className="animate-spin" aria-hidden /> : "Apply"}
                </button>
              </div>
              {phoneRequired && (
                <div style={{ display: "flex", gap: 8, marginTop: 8, alignItems: "center" }}>
                  <div style={{
                    height: 42, padding: "0 12px", borderRadius: "var(--rad-md)",
                    background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
                    display: "flex", alignItems: "center", fontSize: 13, color: "var(--ink-2)", flexShrink: 0,
                  }}>🇮🇳 +91</div>
                  <input
                    type="tel" inputMode="numeric" maxLength={10}
                    value={promoPhone}
                    onChange={(e) => { setPromoPhone(e.target.value.replace(/\D/g, "")); setPromoError(null) }}
                    onKeyDown={(e) => e.key === "Enter" && promoPhone.length === 10 && handleApplyPromo()}
                    placeholder="Phone number for this offer"
                    aria-label="Phone number for this offer"
                    autoFocus
                    style={{
                      flex: 1, height: 42, borderRadius: "var(--rad-md)",
                      background: "var(--bg-elev-1)", border: "1px solid var(--line-2)",
                      padding: "0 12px", fontSize: 13, color: "var(--ink-1)", outline: "none",
                    }}
                  />
                </div>
              )}
              {promoError && (
                <p style={{ marginTop: 6, fontSize: 12, color: "var(--alert)", lineHeight: 1.4 }}>{promoError}</p>
              )}
            </div>
          )}
          {appliedPromo && (
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 12, paddingTop: 10, borderTop: "1px solid var(--line-1)" }}>
              <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>To pay</span>
              <span className="serif" style={{ fontSize: 18, fontWeight: 600, color: "var(--accent)" }}>{formatCurrency(total)}</span>
            </div>
          )}
        </div>
      )}

      {/* Payment methods — host-controlled: only the host requests the bill/payment */}
      {!isHost ? (
        <div style={{ padding: "12px 20px 28px" }}>
          <div
            style={{
              borderRadius: "var(--rad-lg)", padding: "16px 18px",
              background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
              textAlign: "center",
            }}
          >
            <p style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)", marginBottom: 4 }}>
              {hostName ? `${hostName} settles the bill` : "The table host settles the bill"}
            </p>
            <p style={{ fontSize: 12.5, color: "var(--ink-3)", lineHeight: 1.5 }}>
              You can review the bill here — only the host requests payment for the table.
            </p>
          </div>
        </div>
      ) : (
      <div style={{ padding: "12px 20px 28px" }}>
        <span className="eyebrow" style={{ marginBottom: 10, display: "block" }}>Settle up</span>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {PAYMENT_OPTIONS.map(({ method, label, description, icon: Icon }) => {
            const isLoading = loading === method
            const isDisabled = loading !== null || total === 0 || billLoading
            return (
              <button
                key={method}
                onClick={() => handlePay(method)}
                disabled={isDisabled}
                className="press"
                style={{
                  textAlign: "left", padding: "16px",
                  borderRadius: "var(--rad-lg)", background: "var(--bg-elev-1)",
                  border: "1px solid var(--line-1)", boxShadow: "var(--shadow-1)",
                  display: "flex", alignItems: "center", gap: 14,
                  opacity: isDisabled && !isLoading ? 0.4 : 1,
                  transition: "opacity var(--dur-fast) var(--ease)",
                }}
                aria-label={`Pay with ${label}`}
              >
                <span style={{
                  width: 44, height: 44, borderRadius: 14,
                  background: "var(--accent-soft)", color: "var(--accent)",
                  display: "inline-flex", alignItems: "center", justifyContent: "center", flexShrink: 0,
                }}>
                  {isLoading
                    ? <Loader2 style={{ width: 20, height: 20 }} className="animate-spin" aria-hidden />
                    : <Icon style={{ width: 20, height: 20 }} aria-hidden />
                  }
                </span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="serif" style={{ fontSize: 15, fontWeight: 500, color: "var(--ink-1)" }}>{label}</div>
                  <div style={{ fontSize: 13, color: "var(--ink-3)", marginTop: 2, lineHeight: 1.5 }}>{description}</div>
                </div>
                <ChevronRight style={{ width: 16, height: 16, color: "var(--ink-3)", flexShrink: 0 }} aria-hidden />
              </button>
            )
          })}
        </div>

        {total === 0 && !billLoading && !billError && (
          <p style={{ marginTop: 14, fontSize: 13, color: "var(--ink-3)", textAlign: "center", fontStyle: "italic" }}>
            Place an order first to settle up.
          </p>
        )}
      </div>
      )}
    </div>
  )
}
