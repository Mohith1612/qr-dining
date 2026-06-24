"use client"

import { useState, useEffect } from "react"
import { useOrdersStore } from "@/store/orders"
import { useSession } from "@/hooks/useSession"
import { paymentsApi } from "@/lib/api/payments"
import { generateIdempotencyKey } from "@/lib/idempotency"
import { formatCurrency } from "@/lib/format"
import { Banknote, CreditCard, Smartphone, CheckCircle, Loader2, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import type { PaymentMethod, BillData } from "@/types/api"
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
  const { session, participant } = useSession()
  const [bill, setBill] = useState<BillData | null>(null)
  const [billLoading, setBillLoading] = useState(true)
  const [billError, setBillError] = useState<string | null>(null)
  const [loading, setLoading] = useState<PaymentMethod | null>(null)
  const [paid, setPaid] = useState<PaymentMethod | null>(null)
  const [paidTotal, setPaidTotal] = useState(0)
  const [showOptIn, setShowOptIn] = useState(false)

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
    if (!paid) return
    const timer = setTimeout(() => setShowOptIn(true), 1500)
    return () => clearTimeout(timer)
  }, [paid])

  const total = bill?.total ?? 0

  async function handlePay(method: PaymentMethod) {
    if (!session || paid) return
    const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
    setLoading(method)
    try {
      // Send amount=0 as a hint; backend computes the authoritative total server-side.
      const payment = await paymentsApi.initiate(session.id, total, method, generateIdempotencyKey(), undefined, guestToken)
      setPaidTotal(total)
      setPaid(method)
      toast.success(payment.status === "completed" ? "Payment completed." : "Payment request sent.")
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't process payment. Please try again.")
    } finally {
      setLoading(null)
    }
  }

  if (paid) {
    return (
      <>
      <div
        className="flex-1 flex flex-col items-center justify-center px-7 text-center screen-enter"
        style={{ background: "var(--bg-base)" }}
      >
        <div style={{
          width: 84, height: 84, borderRadius: 999,
          background: "var(--ok-soft)", border: "1px solid var(--ok)",
          display: "flex", alignItems: "center", justifyContent: "center",
          boxShadow: "0 0 0 8px var(--ok-soft), 0 12px 30px -10px rgba(0,0,0,0.4)",
          marginBottom: 18,
        }}>
          <CheckCircle style={{ width: 36, height: 36, color: "var(--ok)" }} aria-hidden />
        </div>

        <p className="eyebrow" style={{ marginBottom: 6 }}>Payment recorded</p>
        <h2 className="serif" style={{ margin: 0, fontSize: 32, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.02em" }}>
          Thank you
        </h2>
        <p style={{ margin: "10px 0 22px", color: "var(--ink-2)", fontSize: 14, lineHeight: 1.6, maxWidth: 300 }}>
          {paid === "cash"
            ? `A host will be with you shortly to collect ${formatCurrency(paidTotal)}.`
            : paid === "card"
            ? `Our team is bringing a card terminal for ${formatCurrency(paidTotal)}. Please remain seated.`
            : `We're awaiting confirmation of your ${formatCurrency(paidTotal)} UPI transfer.`}
        </p>

        <HospitalityCard elev={1} style={{ padding: "12px 16px", marginBottom: 22, minWidth: 240, textAlign: "left" }}>
          <Row label="Method" value={METHOD_LABEL[paid]} />
          <Row label="Amount" value={paidTotal > 0 ? formatCurrency(paidTotal) : "—"} />
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

      {/* Payment methods */}
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
    </div>
  )
}
