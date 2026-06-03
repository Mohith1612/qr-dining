"use client"

import { useState, useEffect } from "react"
import { useOrders } from "@/hooks/useOrders"
import { useSession } from "@/hooks/useSession"
import { paymentsApi } from "@/lib/api/payments"
import { formatCurrency } from "@/lib/format"
import { Banknote, CreditCard, Smartphone, CheckCircle, Loader2, ChevronRight } from "lucide-react"
import { toast } from "sonner"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import type { PaymentMethod } from "@/types/api"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { CustomerOptIn } from "@/components/shared/CustomerOptIn"

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

const METHOD_LABEL: Record<PaymentMethod, string> = { cash: "Cash", card: "Card", digital: "UPI" }

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "6px 0" }}>
      <span style={{ color: "var(--ink-3)", fontSize: 13 }}>{label}</span>
      <span style={{ color: "var(--ink-1)", fontSize: 13, fontWeight: 500 }}>{value}</span>
    </div>
  )
}

export default function PaymentPage() {
  const { orders } = useOrders()
  const { session, participant } = useSession()
  const [loading, setLoading] = useState<PaymentMethod | null>(null)
  const [paid, setPaid] = useState<PaymentMethod | null>(null)
  const [showOptIn, setShowOptIn] = useState(false)

  useEffect(() => {
    if (!paid) return
    const timer = setTimeout(() => setShowOptIn(true), 1500)
    return () => clearTimeout(timer)
  }, [paid])

  const total = orders
    .filter((o) => o.status !== "cancelled")
    .reduce((sum, o) => sum + parseFloat(o.total_amount), 0)

  async function handlePay(method: PaymentMethod) {
    if (!session || paid) return
    setLoading(method)
    try {
      await paymentsApi.initiate(session.id, total, method)
      setPaid(method)
      toast.success("Payment recorded — enjoy your meal!")
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
            ? `A host will be with you shortly to collect ${formatCurrency(total)}.`
            : paid === "card"
            ? `Our team is bringing a card terminal for ${formatCurrency(total)}. Please remain seated.`
            : `We're awaiting confirmation of your ${formatCurrency(total)} UPI transfer.`}
        </p>

        <HospitalityCard elev={1} style={{ padding: "12px 16px", marginBottom: 22, minWidth: 240, textAlign: "left" }}>
          <Row label="Method" value={METHOD_LABEL[paid]} />
          <Row label="Amount" value={total > 0 ? formatCurrency(total) : "—"} />
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

      {showOptIn && session && participant && (
        <CustomerOptIn
          sessionId={session.id}
          participantId={participant.id}
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
        <span className="eyebrow">Settle up</span>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          Pay your bill
        </h1>
        <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 13 }}>Choose a method — we'll do the rest.</p>
      </div>

      {/* Amount card */}
      <div style={{ padding: "0 20px 16px" }}>
        <HospitalityCard elev={3} style={{ padding: "20px 22px", background: "linear-gradient(160deg, var(--bg-elev-3), var(--bg-elev-2))" }}>
          <span className="eyebrow">Amount due</span>
          <div className="serif" style={{ marginTop: 4, fontSize: 44, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.02em", lineHeight: 1.05 }}>
            {total > 0 ? formatCurrency(total) : "—"}
          </div>
          <div style={{ marginTop: 8, color: "var(--ink-3)", fontSize: 12, display: "flex", justifyContent: "space-between" }}>
            <span>Includes service &amp; taxes</span>
            {session?.table_identifier && <span>Table {session.table_identifier}</span>}
          </div>
        </HospitalityCard>
      </div>

      {/* Payment methods */}
      <div style={{ padding: "0 20px 28px" }}>
        <span className="eyebrow" style={{ marginBottom: 10, display: "block" }}>Payment method</span>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {PAYMENT_OPTIONS.map(({ method, label, description, icon: Icon }) => {
            const isLoading = loading === method
            return (
              <button
                key={method}
                onClick={() => handlePay(method)}
                disabled={loading !== null}
                className="press"
                style={{
                  textAlign: "left", padding: "16px",
                  borderRadius: "var(--rad-lg)", background: "var(--bg-elev-1)",
                  border: "1px solid var(--line-1)", boxShadow: "var(--shadow-1)",
                  display: "flex", alignItems: "center", gap: 14,
                  opacity: loading !== null && !isLoading ? 0.5 : 1,
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
      </div>
    </div>
  )
}
