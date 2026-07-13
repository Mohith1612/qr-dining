"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { SupportPaymentDetail } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PageHeader, Section, PlatformStatusBadge, PlatformLoading } from "@/components/platform/ui"
import { ArrowLeft } from "lucide-react"
import { toast } from "sonner"

function fmt(ts: string | null | undefined): string {
  if (!ts) return "—"
  try { return new Date(ts).toLocaleString("en", { dateStyle: "medium", timeStyle: "short" }) } catch { return ts }
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <span className="eyebrow">{label}</span>
      <span style={{ fontSize: 14, color: "var(--ink-1)" }}>{value}</span>
    </div>
  )
}

export default function SupportPaymentDetailPage() {
  const params = useParams<{ id: string }>()
  const paymentId = params.id
  const token = usePlatformStore((s) => s.token)
  const [d, setD] = useState<SupportPaymentDetail | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token || !paymentId) return
    setLoading(true)
    try {
      setD(await platformApi.getSupportPayment(paymentId, token))
    } catch {
      toast.error("Couldn't load payment.")
    } finally {
      setLoading(false)
    }
  }, [token, paymentId])

  useEffect(() => { load() }, [load])

  if (loading) return <PlatformLoading />
  if (!d) return <p style={{ color: "var(--ink-3)" }}>Payment not found.</p>

  const p = d.payment
  const bs = d.bill_snapshot
  return (
    <div>
      <Link href={`/platform/support/sessions/${d.session_id}`} style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> Session {d.session_number}
      </Link>
      <PageHeader
        title={p.payment_reference}
        subtitle={`${d.organization.code} · ${d.branch.branch_code}`}
        actions={<PlatformStatusBadge status={p.status} />}
      />

      <Section title="Payment">
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))", gap: 14 }}>
          <Field label="Amount" value={`${p.currency} ${p.amount}`} />
          <Field label="Provider" value={p.provider || "—"} />
          <Field label="Provider payment ref" value={<span className="mono" style={{ fontSize: 12 }}>{p.provider_payment_ref || "—"}</span>} />
          <Field label="Provider order ref" value={<span className="mono" style={{ fontSize: 12 }}>{p.provider_order_ref || "—"}</span>} />
          <Field label="Initiated" value={fmt(p.initiated_at)} />
          <Field label="Completed" value={fmt(p.completed_at)} />
          <Field label="Settled by staff" value={p.settled_by_staff_id ?? "—"} />
          <Field label="Settled at" value={fmt(p.settled_at)} />
        </HospitalityCard>
      </Section>

      <Section title="Bill snapshot">
        {bs ? (
          <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: 14 }}>
            <Field label="Subtotal" value={`₹${bs.subtotal}`} />
            <Field label="Discount" value={`₹${bs.discount_amount}`} />
            <Field label="Tax" value={`₹${bs.tax_amount}`} />
            <Field label="Service charge" value={`₹${bs.service_charge}`} />
            <Field label="Tip" value={`₹${bs.tip_amount}`} />
            <Field label="Total" value={<strong>₹{bs.total}</strong>} />
            <Field label="Finalized by" value={bs.created_by_actor} />
            <Field label="Captured at" value={fmt(bs.created_at)} />
          </HospitalityCard>
        ) : <p style={{ color: "var(--ink-3)", fontSize: 13 }}>No bill snapshot.</p>}
      </Section>

      <Section title={`Webhook history (${d.webhooks.length})`}>
        <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
          {d.webhooks.map((w, i) => (
            <div key={w.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderTop: i === 0 ? "none" : "1px solid var(--line-1)" }}>
              <div>
                <span style={{ fontSize: 13, color: "var(--ink-1)" }}>{w.provider} · {w.event_type}</span>
                {w.error_message && <div style={{ fontSize: 12, color: "var(--alert)" }}>{w.error_message}</div>}
                <div className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{w.external_event_id}</div>
              </div>
              <div style={{ textAlign: "right" }}>
                <span style={{ fontSize: 11, fontWeight: 600, color: w.processed ? "var(--ok)" : "var(--warn)" }}>{w.processed ? "processed" : "pending"}</span>
                <div style={{ fontSize: 11, color: "var(--ink-4)" }}>{fmt(w.created_at)}</div>
              </div>
            </div>
          ))}
          {d.webhooks.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13, padding: "12px 18px" }}>No webhook events.</p>}
        </HospitalityCard>
      </Section>
    </div>
  )
}
