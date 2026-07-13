"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { SupportOrderDetail } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PageHeader, Section, PlatformStatusBadge, PlatformLoading } from "@/components/platform/ui"
import { ArrowLeft } from "lucide-react"
import { toast } from "sonner"

function fmt(ts: string | null | undefined): string {
  if (!ts) return "—"
  try { return new Date(ts).toLocaleString("en", { dateStyle: "medium", timeStyle: "short" }) } catch { return ts }
}

export default function SupportOrderDetailPage() {
  const params = useParams<{ id: string }>()
  const orderId = params.id
  const token = usePlatformStore((s) => s.token)
  const [d, setD] = useState<SupportOrderDetail | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token || !orderId) return
    setLoading(true)
    try {
      setD(await platformApi.getSupportOrder(orderId, token))
    } catch {
      toast.error("Couldn't load order.")
    } finally {
      setLoading(false)
    }
  }, [token, orderId])

  useEffect(() => { load() }, [load])

  if (loading) return <PlatformLoading />
  if (!d) return <p style={{ color: "var(--ink-3)" }}>Order not found.</p>

  const o = d.order
  return (
    <div>
      <Link href={`/platform/support/sessions/${d.session_id}`} style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> Session {d.session_number}
      </Link>
      <PageHeader
        title={o.order_operational_id || o.order_number_display}
        subtitle={`${d.organization.code} · ${d.branch.branch_code}`}
        actions={<PlatformStatusBadge status={o.status} />}
      />

      <Section title="Order">
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 14 }}>
          <div><span className="eyebrow">Order ID</span><div className="mono" style={{ fontSize: 12, color: "var(--ink-1)" }}>{o.id}</div></div>
          <div><span className="eyebrow">Display #</span><div style={{ fontSize: 14, color: "var(--ink-1)" }}>{o.order_number_display}</div></div>
          <div><span className="eyebrow">Total</span><div style={{ fontSize: 14, color: "var(--ink-1)" }}>₹{o.total_amount}</div></div>
          <div><span className="eyebrow">Discount</span><div style={{ fontSize: 14, color: "var(--ink-1)" }}>₹{o.discount_amount}</div></div>
          <div><span className="eyebrow">Created</span><div style={{ fontSize: 14, color: "var(--ink-1)" }}>{fmt(o.created_at)}</div></div>
          <div><span className="eyebrow">Updated</span><div style={{ fontSize: 14, color: "var(--ink-1)" }}>{fmt(o.updated_at)}</div></div>
        </HospitalityCard>
      </Section>

      <Section title={`Items (${o.items?.length ?? 0})`}>
        <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
          {(o.items ?? []).map((it, i) => (
            <div key={it.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderTop: i === 0 ? "none" : "1px solid var(--line-1)" }}>
              <div>
                <span style={{ fontSize: 13, color: "var(--ink-1)" }}>Menu item #{it.menu_item_id} × {it.quantity}</span>
                {it.note && <div style={{ fontSize: 12, color: "var(--ink-4)" }}>note: {it.note}</div>}
              </div>
              <span style={{ fontSize: 13, color: "var(--ink-2)" }}>₹{it.unit_price}</span>
            </div>
          ))}
          {(o.items?.length ?? 0) === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13, padding: "12px 18px" }}>No items.</p>}
        </HospitalityCard>
      </Section>
    </div>
  )
}
