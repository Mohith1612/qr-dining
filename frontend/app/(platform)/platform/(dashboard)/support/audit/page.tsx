"use client"

import { Suspense, useCallback, useEffect, useState } from "react"
import { useSearchParams } from "next/navigation"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { AuditEvent, Organization } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Input } from "@/components/ui/input"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { ShieldAlert } from "lucide-react"
import { toast } from "sonner"

const RESULT_TONE: Record<string, string> = {
  success: "var(--ok)", failure: "var(--warn)", denied: "var(--alert)",
}
const RISK_TONE: Record<string, string> = {
  low: "var(--ink-4)", medium: "var(--info)", high: "var(--warn)", critical: "var(--alert)",
}

function fmt(ts: string): string {
  try { return new Date(ts).toLocaleString("en", { dateStyle: "medium", timeStyle: "short" }) } catch { return ts }
}

export default function SupportAuditPage() {
  return (
    <Suspense fallback={<PlatformLoading />}>
      <AuditInner />
    </Suspense>
  )
}

function AuditInner() {
  const searchParams = useSearchParams()
  const { token, roles } = usePlatformStore()
  // Audit reads require the read_only_auditor role on the backend.
  const canAudit = hasPlatformRole(roles, "read_only_auditor")

  const sessionFilter = searchParams.get("session") ?? ""
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [orgFilter, setOrgFilter] = useState<number | undefined>(undefined)
  const [resultFilter, setResultFilter] = useState<string>("")
  const [events, setEvents] = useState<AuditEvent[] | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!token || !canAudit) return
    platformApi.listOrganizations(token).then((r) => setOrgs(r.organizations)).catch(() => {})
  }, [token, canAudit])

  const load = useCallback(async () => {
    if (!token || !canAudit) return
    setLoading(true)
    try {
      const { audit } = await platformApi.listAudit(
        {
          organization_id: orgFilter,
          session_id: sessionFilter || undefined,
          result: resultFilter || undefined,
        },
        token
      )
      setEvents(audit)
    } catch {
      toast.error("Couldn't load audit events.")
    } finally {
      setLoading(false)
    }
  }, [token, canAudit, orgFilter, sessionFilter, resultFilter])

  useEffect(() => { load() }, [load])

  if (!canAudit) {
    return (
      <div>
        <PageHeader title="Audit" />
        <HospitalityCard elev={1} style={{ padding: "20px", display: "flex", alignItems: "center", gap: 12 }}>
          <ShieldAlert size={18} style={{ color: "var(--ink-4)" }} aria-hidden />
          <p style={{ fontSize: 14, color: "var(--ink-2)", margin: 0 }}>Audit access requires the read_only_auditor role.</p>
        </HospitalityCard>
      </div>
    )
  }

  return (
    <div>
      <PageHeader title="Audit" subtitle={sessionFilter ? `Filtered to session ${sessionFilter}` : "Immutable platform audit log"} />

      <div style={{ display: "flex", gap: 12, flexWrap: "wrap", marginBottom: 16, alignItems: "flex-end" }}>
        <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 240 }}>
          <span className="eyebrow">Organization</span>
          <select
            value={orgFilter ?? ""}
            onChange={(e) => setOrgFilter(e.target.value ? Number(e.target.value) : undefined)}
            style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
          >
            <option value="">All organizations</option>
            {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
          </select>
        </label>
        <label style={{ display: "flex", flexDirection: "column", gap: 6, width: 160 }}>
          <span className="eyebrow">Result</span>
          <select
            value={resultFilter}
            onChange={(e) => setResultFilter(e.target.value)}
            style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
          >
            <option value="">All results</option>
            <option value="success">Success</option>
            <option value="failure">Failure</option>
            <option value="denied">Denied</option>
          </select>
        </label>
        {sessionFilter && (
          <label style={{ display: "flex", flexDirection: "column", gap: 6, flex: 1, minWidth: 200 }}>
            <span className="eyebrow">Session filter (from link)</span>
            <Input value={sessionFilter} disabled style={{ fontFamily: "var(--font-mono, monospace)", fontSize: 12 }} />
          </label>
        )}
      </div>

      <Section title={events ? `Events (${events.length})` : "Events"}>
        {loading ? <PlatformLoading /> : (
          <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
            {(events ?? []).map((e, i) => (
              <div key={e.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderTop: i === 0 ? "none" : "1px solid var(--line-1)" }}>
                <div style={{ minWidth: 0 }}>
                  <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{e.action}</span>
                  <div style={{ fontSize: 12, color: "var(--ink-4)" }}>
                    {e.actor_type}{e.actor_display ? ` (${e.actor_display})` : ""} · {e.resource_type}{e.resource_id ? `#${e.resource_id}` : ""}
                  </div>
                </div>
                <div style={{ textAlign: "right", flexShrink: 0 }}>
                  <span style={{ fontSize: 11, fontWeight: 600, color: RESULT_TONE[e.result] ?? "var(--ink-3)" }}>{e.result}</span>
                  <span style={{ fontSize: 10.5, marginLeft: 8, color: RISK_TONE[e.risk_level] ?? "var(--ink-4)" }}>{e.risk_level}</span>
                  <div style={{ fontSize: 11, color: "var(--ink-4)" }}>{fmt(e.created_at)}</div>
                </div>
              </div>
            ))}
            {events !== null && events.length === 0 && <p style={{ color: "var(--ink-3)", fontSize: 13, padding: "12px 18px" }}>No audit events match.</p>}
          </HospitalityCard>
        )}
      </Section>
    </div>
  )
}
