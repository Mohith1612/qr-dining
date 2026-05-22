"use client"

import { useState } from "react"
import { useOrders } from "@/hooks/useOrders"
import { useSession } from "@/hooks/useSession"
import { paymentsApi } from "@/lib/api/payments"
import { formatCurrency } from "@/lib/format"
import { Banknote, CreditCard, Smartphone, CheckCircle, Loader2 } from "lucide-react"
import { toast } from "sonner"
import type { PaymentMethod } from "@/types/api"

const PAYMENT_OPTIONS: {
  method: PaymentMethod
  label: string
  description: string
  icon: React.ElementType
}[] = [
  {
    method: "cash",
    label: "Cash",
    description: "Pay with cash — your waiter will collect",
    icon: Banknote,
  },
  {
    method: "card",
    label: "Card",
    description: "Credit or debit card via POS terminal",
    icon: CreditCard,
  },
  {
    method: "digital",
    label: "Digital / UPI",
    description: "UPI, Google Pay, PhonePe, and more",
    icon: Smartphone,
  },
]

const METHOD_LABEL: Record<PaymentMethod, string> = {
  cash: "Cash",
  card: "Card",
  digital: "UPI",
}

export default function PaymentPage() {
  const { orders } = useOrders()
  const { session } = useSession()
  const [loading, setLoading] = useState<PaymentMethod | null>(null)
  const [paid, setPaid] = useState<PaymentMethod | null>(null)

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
    } catch {
      toast.error("Couldn't process payment. Please try again.")
    } finally {
      setLoading(null)
    }
  }

  if (paid) {
    return (
      <div
        className="min-h-[60vh] flex flex-col items-center justify-center px-6 text-center gap-5"
        style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
      >
        <div
          className="size-16 rounded-full flex items-center justify-center"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
        >
          <CheckCircle className="size-8" style={{ color: "var(--color-success)" }} aria-hidden />
        </div>
        <div className="space-y-1.5">
          <p className="text-lg font-semibold">Payment recorded</p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            {METHOD_LABEL[paid]} — {total > 0 ? formatCurrency(total) : "your bill"}
          </p>
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Your waiter will come to complete the transaction. Thank you!
          </p>
        </div>
      </div>
    )
  }

  return (
    <div
      className="px-4 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div className="space-y-1">
        <h1 className="text-lg font-semibold">Pay bill</h1>
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          Choose how you'd like to pay.
        </p>
      </div>

      <div
        className="rounded-2xl p-5 flex items-center justify-between"
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
        }}
      >
        <span className="text-sm font-medium" style={{ color: "var(--color-text-muted)" }}>
          Total
        </span>
        <span className="text-2xl font-bold" style={{ color: "var(--color-text)" }}>
          {total > 0 ? formatCurrency(total) : "—"}
        </span>
      </div>

      {total === 0 && (
        <p className="text-xs text-center" style={{ color: "var(--color-text-muted)" }}>
          No orders placed yet. The total will update as orders are confirmed.
        </p>
      )}

      <section className="space-y-3">
        <h2
          className="text-xs font-semibold uppercase tracking-wider"
          style={{ color: "var(--color-text-muted)" }}
        >
          Payment method
        </h2>

        {PAYMENT_OPTIONS.map(({ method, label, description, icon: Icon }) => {
          const isLoading = loading === method
          return (
            <button
              key={method}
              onClick={() => handlePay(method)}
              disabled={loading !== null}
              className="w-full flex items-center gap-4 p-4 rounded-2xl text-left transition-opacity active:opacity-70 disabled:cursor-not-allowed"
              style={{
                backgroundColor: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                boxShadow: "var(--shadow-card)",
                borderRadius: "var(--radius-lg)",
                minHeight: "72px",
              }}
              aria-label={`Pay with ${label}`}
            >
              <div
                className="size-10 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "var(--color-bg)" }}
              >
                {isLoading ? (
                  <Loader2 className="size-5 animate-spin" style={{ color: "var(--color-accent)" }} aria-hidden />
                ) : (
                  <Icon className="size-5" style={{ color: "var(--color-accent)" }} aria-hidden />
                )}
              </div>
              <div className="space-y-0.5">
                <p className="font-semibold text-sm">{label}</p>
                <p className="text-xs leading-relaxed" style={{ color: "var(--color-text-muted)" }}>
                  {description}
                </p>
              </div>
            </button>
          )
        })}
      </section>
    </div>
  )
}
