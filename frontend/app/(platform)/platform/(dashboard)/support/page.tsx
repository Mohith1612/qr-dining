"use client"

import { useCallback, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { SupportSearchResult, Organization, UsageReport, HealthReport } from "@/types/platform"
import type { AnalyticsPeriod } from "@/lib/api/analytics"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Input } from "@/components/ui/input"
import { PageHeader, Section, StatCard, PlatformLoading, PlatformStatusBadge, sumCounts } from "@/components/platform/ui"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { Search, ChevronRight, ShieldAlert } from "lucide-react"
import { toast } from "sonner"

const TYPE_ORDER = ["organization", "branch", "table", "session", "participant", "order", "payment", "audit_event"] as const
const TYPE_LABEL: Record<string, string> = {
  organization: "Organizations", branch: "Branches", table: "Tables", session: "Sessions",
  participant: "Participants", order: "Orders", payment: "Payments", audit_event: "Audit events",
}

function resultHref(r: SupportSearchResult): string | null {
  switch (r.type) {
    case "session": return `/platform/support/sessions/${r.id}`
    case "order": return `/platform/support/orders/${r.id}`
    case "payment": return `/platform/support/payments/${r.id}`
    case "participant": return r.related_session_id ? `/platform/support/sessions/${r.related_session_id}` : null
    case "organization": return `/platform/organizations/${r.id}`
    case "branch": return r.organization_id ? `/platform/organizations/${r.organization_id}` : null
    case "audit_event": return "/platform/support/audit"
    default: return null
  }
}

export default function SupportConsolePage() {
  const router = useRouter()
  const { token, roles } = usePlatformStore()
  const canSupport = hasPlatformRole(roles, "support_admin", "read_only_auditor")

  const [query, setQuery] = useState("")
  const [results, setResults] = useState<SupportSearchResult[] | null>(null)
  const [searching, setSearching] = useState(false)

  // Tenant health
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [healthOrg, setHealthOrg] = useState<number | undefined>(undefined)
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [health, setHealth] = useState<HealthReport | null>(null)
  const [healthLoading, setHealthLoading] = useState(false)

  useEffect(() => {
    if (!token || !canSupport) return
    platformApi.listOrganizations(token).then((r) => setOrgs(r.organizations)).catch(() => {})
  }, [token, canSupport])

  const runSearch = useCallback(async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (!token || !query.trim()) return
    setSearching(true)
    try {
      const { results } = await platformApi.supportSearch(query.trim(), token)
      setResults(results)
    } catch {
      toast.error("Search failed.")
    } finally {
      setSearching(false)
    }
  }, [token, query])

  const loadHealth = useCallback(async () => {
    if (!token || !healthOrg) { setUsage(null); setHealth(null); return }
    setHealthLoading(true)
    try {
      const [u, h] = await Promise.all([
        platformApi.getUsage(period, healthOrg, token),
        platformApi.getHealth(period, healthOrg, token),
      ])
      setUsage(u); setHealth(h)
    } catch {
      toast.error("Couldn't load tenant health.")
    } finally {
      setHealthLoading(false)
    }
  }, [token, healthOrg, period])

  useEffect(() => { loadHealth() }, [loadHealth])

  if (!canSupport) {
    return (
      <div>
        <PageHeader title="Support" />
        <HospitalityCard elev={1} style={{ padding: "20px", display: "flex", alignItems: "center", gap: 12 }}>
          <ShieldAlert size={18} style={{ color: "var(--ink-4)" }} aria-hidden />
          <p style={{ fontSize: 14, color: "var(--ink-2)", margin: 0 }}>
            Support access requires the support_admin or read_only_auditor role.
          </p>
        </HospitalityCard>
      </div>
    )
  }

  const grouped = TYPE_ORDER.map((t) => ({ type: t, rows: (results ?? []).filter((r) => r.type === t) })).filter((g) => g.rows.length > 0)

  return (
    <div>
      <PageHeader title="Support" subtitle="Read-only tenant diagnostics" />

      <form onSubmit={runSearch} style={{ position: "relative", maxWidth: 520, marginBottom: 8 }}>
        <Search size={15} style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)", color: "var(--ink-4)" }} aria-hidden />
        <Input
          placeholder="Search session / order / payment refs, org or branch names, table, participant…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={{ paddingLeft: 32 }}
        />
      </form>
      <p style={{ fontSize: 12, color: "var(--ink-4)", marginBottom: 20 }}>Press Enter to search. All lookups are audit-logged.</p>

      {searching ? <PlatformLoading /> : results !== null && (
        <Section title={`Results (${results.length})`}>
          {grouped.length === 0 ? (
            <p style={{ color: "var(--ink-3)", fontSize: 14 }}>No matches.</p>
          ) : grouped.map((g) => (
            <div key={g.type} style={{ marginBottom: 16 }}>
              <p className="eyebrow" style={{ marginBottom: 8 }}>{TYPE_LABEL[g.type]} ({g.rows.length})</p>
              <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                {g.rows.map((r) => {
                  const href = resultHref(r)
                  const card = (
                    <HospitalityCard elev={1} press={!!href} style={{ padding: "12px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
                          <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>{r.label || r.reference}</span>
                          {r.reference && r.reference !== r.label && <span className="mono" style={{ fontSize: 12, color: "var(--ink-4)" }}>{r.reference}</span>}
                        </div>
                        <span style={{ fontSize: 12, color: "var(--ink-4)" }}>
                          {r.organization_code}{r.branch_code ? ` · ${r.branch_code}` : ""}
                        </span>
                      </div>
                      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                        {r.status && <PlatformStatusBadge status={r.status} />}
                        {href && <ChevronRight size={16} style={{ color: "var(--ink-4)" }} aria-hidden />}
                      </div>
                    </HospitalityCard>
                  )
                  return href ? (
                    <div key={`${r.type}-${r.id}`} onClick={() => router.push(href)} style={{ cursor: "pointer" }}>{card}</div>
                  ) : (
                    <div key={`${r.type}-${r.id}`}>{card}</div>
                  )
                })}
              </div>
            </div>
          ))}
        </Section>
      )}

      <Section
        title="Tenant health"
        actions={healthOrg ? <PeriodSelector value={period} onChange={setPeriod} /> : undefined}
      >
        <label style={{ display: "flex", flexDirection: "column", gap: 6, maxWidth: 360, marginBottom: 14 }}>
          <span className="eyebrow">Organization</span>
          <select
            value={healthOrg ?? ""}
            onChange={(e) => setHealthOrg(e.target.value ? Number(e.target.value) : undefined)}
            style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
          >
            <option value="">Select an organization…</option>
            {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
          </select>
        </label>

        {healthOrg && (healthLoading ? <PlatformLoading /> : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12 }}>
            <StatCard label="Active branches" value={usage?.active_branches ?? 0} />
            <StatCard label="Active diners" value={usage?.active_diners ?? 0} hint="right now" />
            <StatCard label="Sessions" value={usage ? sumCounts(usage.sessions_per_day) : 0} hint="in period" />
            <StatCard label="Payments" value={usage ? sumCounts(usage.payments_per_day) : 0} hint="in period" />
            <StatCard label="Webhook failures" value={health ? sumCounts(health.webhook_failures_per_day) : 0} hint="in period" />
            <StatCard label="Authz denials" value={health ? sumCounts(health.authz_denials_per_day) : 0} hint="in period" />
          </div>
        ))}
      </Section>
    </div>
  )
}
