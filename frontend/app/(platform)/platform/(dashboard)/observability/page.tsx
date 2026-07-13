"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { SubscriptionObservability, EntitlementObservability, FlagObservability } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { toast } from "sonner"

const REASON_TONE: Record<string, { bg: string; fg: string; label: string }> = {
  suspended: { bg: "var(--alert-soft)", fg: "var(--alert)", label: "suspended" },
  expired: { bg: "var(--alert-soft)", fg: "var(--alert)", label: "expired" },
  past_due: { bg: "var(--alert-soft)", fg: "var(--alert)", label: "past due" },
  trial_expired: { bg: "var(--alert-soft)", fg: "var(--alert)", label: "trial expired" },
  trial_ending: { bg: "var(--warn-soft, var(--accent-soft))", fg: "var(--warn, var(--accent))", label: "trial ending" },
}

export default function ObservabilityPage() {
  const token = usePlatformStore((s) => s.token)
  const [subs, setSubs] = useState<SubscriptionObservability | null>(null)
  const [ent, setEnt] = useState<EntitlementObservability | null>(null)
  const [flags, setFlags] = useState<FlagObservability | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    try {
      const [s, e, f] = await Promise.all([
        platformApi.getSubscriptionObservability(token),
        platformApi.getEntitlementObservability(token),
        platformApi.getFlagObservability(token),
      ])
      setSubs(s); setEnt(e); setFlags(f)
    } catch {
      toast.error("Couldn't load observability.")
    } finally {
      setLoading(false)
    }
  }, [token])
  useEffect(() => { load() }, [load])

  if (loading) return <PlatformLoading />

  return (
    <div>
      <PageHeader
        title="Enforcement observability"
        subtitle="Where enforcement would bite if it were turned on — observe before enforcing. Nothing here gates any tenant."
      />

      {/* Subscriptions */}
      <Section title={`Subscriptions needing attention (${subs?.alerts.length ?? 0} of ${subs?.total ?? 0})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {(!subs || subs.alerts.length === 0) && <Empty text="All subscriptions healthy." />}
          {subs?.alerts.map((a) => {
            const tone = REASON_TONE[a.reason] ?? { bg: "var(--line-2)", fg: "var(--ink-3)", label: a.reason }
            return (
              <Row key={`${a.organization_id}-${a.reason}`} orgId={a.organization_id}>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{a.org_name}</span>
                  <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{a.org_code}</span>
                  <Pill bg={tone.bg} fg={tone.fg}>{tone.label}</Pill>
                </div>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                  {a.plan_tier} · {a.trial_ends_at ? `trial ends ${fmt(a.trial_ends_at)}` : a.expires_at ? `expires ${fmt(a.expires_at)}` : a.status}
                </span>
              </Row>
            )
          })}
        </div>
      </Section>

      {/* Entitlement limit breaches */}
      <Section title={`Entitlement limit breaches (${ent?.breaches.length ?? 0}; ${ent?.checked ?? 0} orgs checked)`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {(!ent || ent.breaches.length === 0) && <Empty text="No tenant is over its resolved limits." />}
          {ent?.breaches.map((b, i) => (
            <Row key={`${b.organization_id}-${b.key}-${i}`} orgId={b.organization_id}>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{b.org_name}</span>
                <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)" }}>{b.key}</span>
              </div>
              <span style={{ fontSize: 12 }}>
                <strong style={{ color: "var(--alert)" }}>{b.actual}</strong>
                <span style={{ color: "var(--ink-4)" }}> / limit {b.limit}</span>
              </span>
            </Row>
          ))}
        </div>
      </Section>

      {/* Feature flags */}
      <Section title={`Feature-flag overrides in use (${flags?.in_use.length ?? 0})`}>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {(!flags || flags.in_use.length === 0) && <Empty text="No feature-flag overrides in use." />}
          {flags?.in_use.map((s) => (
            <HospitalityCard key={s.key} elev={1} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)", fontWeight: 600 }}>{s.key}</span>
                <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{s.name}</span>
              </div>
              <span style={{ fontSize: 12, color: "var(--ink-3)", display: "flex", gap: 12 }}>
                <span>global <strong style={{ color: "var(--ink-1)" }}>{s.global}</strong></span>
                <span>org <strong style={{ color: "var(--ink-1)" }}>{s.org}</strong></span>
                <span>branch <strong style={{ color: "var(--ink-1)" }}>{s.branch}</strong></span>
              </span>
            </HospitalityCard>
          ))}
        </div>
      </Section>

      {flags && flags.orphaned.length > 0 && (
        <Section title={`Orphaned flags (${flags.orphaned.length})`}>
          <HospitalityCard elev={1} style={{ padding: "12px 16px", display: "flex", flexWrap: "wrap", gap: 8 }}>
            {flags.orphaned.map((k) => (
              <span key={k} className="mono" style={{ fontSize: 11, padding: "3px 8px", borderRadius: "var(--rad-pill)", background: "var(--line-2)", color: "var(--ink-3)" }}>{k}</span>
            ))}
          </HospitalityCard>
        </Section>
      )}
    </div>
  )
}

function Row({ orgId, children }: { orgId: number; children: React.ReactNode }) {
  return (
    <Link href={`/platform/organizations/${orgId}`} style={{ textDecoration: "none" }}>
      <HospitalityCard elev={1} press style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        {children}
      </HospitalityCard>
    </Link>
  )
}

function Pill({ bg, fg, children }: { bg: string; fg: string; children: React.ReactNode }) {
  return <span style={{ fontSize: 11, fontWeight: 500, textTransform: "uppercase", letterSpacing: "0.03em", padding: "3px 8px", borderRadius: "var(--rad-pill)", background: bg, color: fg }}>{children}</span>
}

function Empty({ text }: { text: string }) {
  return <p style={{ color: "var(--ink-3)", fontSize: 14, padding: "8px 0", margin: 0 }}>{text}</p>
}

function fmt(iso: string): string {
  try { return new Date(iso).toLocaleDateString("en", { month: "short", day: "numeric", year: "numeric" }) } catch { return iso }
}
