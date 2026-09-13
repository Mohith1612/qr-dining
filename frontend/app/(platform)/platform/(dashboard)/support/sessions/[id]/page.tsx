"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { SupportSessionDetail } from "@/types/platform"
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

export default function SupportSessionDetailPage() {
  const params = useParams<{ id: string }>()
  const sessionId = params.id
  const token = usePlatformStore((s) => s.token)
  const [d, setD] = useState<SupportSessionDetail | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token || !sessionId) return
    setLoading(true)
    try {
      setD(await platformApi.getSupportSession(sessionId, token))
    } catch {
      toast.error("Couldn't load session.")
    } finally {
      setLoading(false)
    }
  }, [token, sessionId])

  useEffect(() => { load() }, [load])

  if (loading) return <PlatformLoading />
  if (!d) return <p style={{ color: "var(--ink-3)" }}>Session not found.</p>

  return (
    <div>
      <Link href="/platform/support" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> Support
      </Link>
      <PageHeader
        title={d.session.session_number || "Session"}
        subtitle={`${d.organization.code} · ${d.branch.branch_code} · Table ${d.table.identifier}`}
        actions={<PlatformStatusBadge status={d.session.status} />}
      />

      <Section title="Session">
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 14 }}>
          <Field label="Session ID" value={<span className="mono" style={{ fontSize: 12 }}>{d.session.id}</span>} />
          <Field label="Visit" value={d.session.visit_number} />
          <Field label="Created" value={fmt(d.session.created_at)} />
          <Field label="Closed" value={fmt(d.session.closed_at)} />
          <Field label="Timezone" value={d.branch.timezone} />
        </HospitalityCard>
      </Section>

      <Section title={`Participants (${d.participants.length})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {d.participants.map((p) => (
            <HospitalityCard key={p.id} elev={1} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
              <div>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{p.display_name}</span>
                {p.is_host && <span className="mono" style={{ fontSize: 10.5, marginLeft: 8, padding: "2px 6px", borderRadius: "var(--rad-pill)", background: "var(--accent-soft)", color: "var(--accent)" }}>HOST</span>}
                {p.revoked_at && <span style={{ fontSize: 11, marginLeft: 8, color: "var(--alert)" }}>revoked</span>}
                <div style={{ fontSize: 12, color: "var(--ink-4)" }}>{p.phone_e164 || "no phone"} · joined {fmt(p.joined_at)}</div>
              </div>
            </HospitalityCard>
          ))}
          {d.participants.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13 }}>None.</p>}
        </div>
      </Section>

      <Section title={`Orders (${d.orders.length})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {d.orders.map((o) => (
            <Link key={o.id} href={`/platform/support/orders/${o.id}`} style={{ textDecoration: "none" }}>
              <HospitalityCard elev={1} press style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <div>
                  <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{o.order_operational_id || o.order_number_display}</span>
                  <div style={{ fontSize: 12, color: "var(--ink-4)" }}>{o.items?.length ?? 0} item(s) · ₹{o.total_amount}</div>
                </div>
                <PlatformStatusBadge status={o.status} />
              </HospitalityCard>
            </Link>
          ))}
          {d.orders.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13 }}>None.</p>}
        </div>
      </Section>

      <Section title={`Payments (${d.payments.length})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {d.payments.map((p) => (
            <Link key={p.id} href={`/platform/support/payments/${p.id}`} style={{ textDecoration: "none" }}>
              <HospitalityCard elev={1} press style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <div>
                  <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{p.payment_reference}</span>
                  <div style={{ fontSize: 12, color: "var(--ink-4)" }}>{p.currency} {p.amount}{p.provider ? ` · ${p.provider}` : ""}</div>
                </div>
                <PlatformStatusBadge status={p.status} />
              </HospitalityCard>
            </Link>
          ))}
          {d.payments.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13 }}>None.</p>}
        </div>
      </Section>

      {d.assistance.length > 0 && (
        <Section title={`Assistance (${d.assistance.length})`}>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {d.assistance.map((a) => (
              <HospitalityCard key={a.id} elev={1} style={{ padding: "10px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <span style={{ fontSize: 13, color: "var(--ink-1)" }}>{a.type} · {fmt(a.created_at)}</span>
                <PlatformStatusBadge status={a.status} />
              </HospitalityCard>
            ))}
          </div>
        </Section>
      )}

      <Section title={`Lifecycle timeline (${d.timeline.length})`}>
        <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
          {d.timeline.map((e, i) => (
            <div key={e.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "9px 18px", borderTop: i === 0 ? "none" : "1px solid var(--line-1)" }}>
              <span className="mono" style={{ fontSize: 12.5, color: "var(--ink-1)" }}>{e.event_type}</span>
              <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{e.actor_type} · {fmt(e.created_at)}</span>
            </div>
          ))}
          {d.timeline.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13, padding: "12px 18px" }}>No events.</p>}
        </HospitalityCard>
      </Section>

      <p style={{ fontSize: 12, color: "var(--ink-4)", marginTop: 24 }}>
        Read-only view. <Link href={`/platform/support/audit?session=${d.session.id}`} style={{ color: "var(--accent)", textDecoration: "none" }}>View audit events for this session →</Link>
      </p>
    </div>
  )
}
