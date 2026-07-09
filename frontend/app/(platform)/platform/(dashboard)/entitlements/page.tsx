"use client"

import { Suspense, useCallback, useEffect, useState } from "react"
import { useSearchParams } from "next/navigation"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { Organization, PlatformPlan, EffectiveEntitlements, EntitlementCatalogEntry } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"

export default function EntitlementsPage() {
  return (
    <Suspense fallback={<PlatformLoading />}>
      <EntitlementsInner />
    </Suspense>
  )
}

function EntitlementsInner() {
  const searchParams = useSearchParams()
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [orgs, setOrgs] = useState<Organization[]>([])
  const [plans, setPlans] = useState<PlatformPlan[]>([])
  const [catalog, setCatalog] = useState<EntitlementCatalogEntry[]>([])
  const [selectedOrg, setSelectedOrg] = useState<number | null>(null)
  const [ent, setEnt] = useState<EffectiveEntitlements | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  // forms
  const [planId, setPlanId] = useState<number | "">("")
  const [overrideKey, setOverrideKey] = useState("")
  const [overrideEnabled, setOverrideEnabled] = useState(true)
  const [overrideLimit, setOverrideLimit] = useState("")
  const [overrideReason, setOverrideReason] = useState("")

  useEffect(() => {
    if (!token) return
    Promise.all([
      platformApi.listOrganizations(token),
      platformApi.listPlans(token),
      platformApi.listEntitlementCatalog(token),
    ]).then(([o, p, c]) => {
      setOrgs(o.organizations)
      setPlans(p.plans)
      setCatalog(c.entitlements)
      const fromUrl = Number(searchParams.get("org"))
      const initial = fromUrl || o.organizations[0]?.id || null
      setSelectedOrg(initial)
    }).catch(() => toast.error("Couldn't load entitlement data."))
      .finally(() => setLoading(false))
  }, [token, searchParams])

  const loadEnt = useCallback(async () => {
    if (!token || !selectedOrg) return
    try {
      const { entitlements } = await platformApi.getOrganizationEntitlements(selectedOrg, token)
      setEnt(entitlements)
    } catch {
      toast.error("Couldn't resolve entitlements.")
    }
  }, [token, selectedOrg])

  useEffect(() => { loadEnt() }, [loadEnt])

  async function assignPlan() {
    if (!token || !selectedOrg || planId === "") return
    setBusy(true)
    try {
      await platformApi.assignOrganizationPlan(selectedOrg, Number(planId), "active", token)
      toast.success("Plan assigned.")
      await loadEnt()
    } catch {
      toast.error("Couldn't assign plan.")
    } finally {
      setBusy(false)
    }
  }

  async function applyOverride() {
    if (!token || !selectedOrg || !overrideKey) return
    const isLimit = overrideKey.startsWith("limit.")
    setBusy(true)
    try {
      await platformApi.setEntitlementOverride(
        selectedOrg,
        overrideKey,
        isLimit
          ? { limit_value: overrideLimit === "" ? null : Number(overrideLimit), reason: overrideReason }
          : { enabled: overrideEnabled, reason: overrideReason },
        token
      )
      toast.success("Override applied.")
      setOverrideReason("")
      await loadEnt()
    } catch {
      toast.error("Couldn't apply override.")
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <PlatformLoading />

  const selectedOrgObj = orgs.find((o) => o.id === selectedOrg)
  const overrideIsLimit = overrideKey.startsWith("limit.")

  return (
    <div>
      <PageHeader title="Entitlements" subtitle="Resolved organization capabilities & limits" />

      <label style={{ display: "flex", flexDirection: "column", gap: 6, maxWidth: 360, marginBottom: 20 }}>
        <span className="eyebrow">Organization</span>
        <Select value={selectedOrg ?? ""} onChange={(v) => setSelectedOrg(v ? Number(v) : null)}>
          {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
        </Select>
      </label>

      {!ent ? <PlatformLoading /> : (
        <>
          <Section title="Resolved entitlements">
            <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", flexDirection: "column", gap: 14 }}>
              <div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
                <Meta label="Plan tier" value={ent.plan_tier} />
                <Meta label="Source" value={ent.source} />
              </div>
              <div>
                <span className="eyebrow">Capabilities</span>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginTop: 8 }}>
                  {Object.entries(ent.capabilities).map(([k, v]) => (
                    <span key={k} className="mono" style={{
                      fontSize: 11, padding: "3px 8px", borderRadius: "var(--rad-pill)",
                      background: v ? "var(--ok-soft)" : "var(--line-2)",
                      color: v ? "var(--ok)" : "var(--ink-4)",
                    }}>{v ? "✓" : "✕"} {k}</span>
                  ))}
                </div>
              </div>
              <div>
                <span className="eyebrow">Limits</span>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 14, marginTop: 8 }}>
                  {Object.entries(ent.limits).map(([k, v]) => (
                    <span key={k} style={{ fontSize: 13, color: "var(--ink-2)" }}>
                      <span className="mono" style={{ color: "var(--ink-4)" }}>{k}</span>{" "}
                      <strong style={{ color: "var(--ink-1)" }}>{v === -1 ? "∞" : v}</strong>
                    </span>
                  ))}
                </div>
              </div>
            </HospitalityCard>
          </Section>

          {canManage && (
            <Section title="Assign plan">
              <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", gap: 12, alignItems: "flex-end", flexWrap: "wrap" }}>
                <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 200 }}>
                  <span className="eyebrow">Plan</span>
                  <Select value={planId} onChange={(v) => setPlanId(v ? Number(v) : "")}>
                    <option value="">Select a plan…</option>
                    {plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.tier})</option>)}
                  </Select>
                </label>
                <Button onClick={assignPlan} disabled={busy || planId === ""}>
                  {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Assign to {selectedOrgObj?.code}
                </Button>
              </HospitalityCard>
            </Section>
          )}

          {canManage && (
            <Section title="Set override">
              <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", gap: 12, alignItems: "flex-end", flexWrap: "wrap" }}>
                <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 200 }}>
                  <span className="eyebrow">Entitlement</span>
                  <Select value={overrideKey} onChange={(v) => setOverrideKey(v)}>
                    <option value="">Select…</option>
                    {catalog.map((c) => <option key={c.key} value={c.key}>{c.key}</option>)}
                  </Select>
                </label>
                {overrideKey && (overrideIsLimit ? (
                  <label style={{ display: "flex", flexDirection: "column", gap: 6, width: 120 }}>
                    <span className="eyebrow">Limit (∞ = blank)</span>
                    <Input value={overrideLimit} onChange={(e) => setOverrideLimit(e.target.value.replace(/[^0-9]/g, ""))} inputMode="numeric" placeholder="∞" />
                  </label>
                ) : (
                  <label style={{ display: "flex", flexDirection: "column", gap: 6, width: 140 }}>
                    <span className="eyebrow">Enabled</span>
                    <Select value={overrideEnabled ? "1" : "0"} onChange={(v) => setOverrideEnabled(v === "1")}>
                      <option value="1">Enabled</option>
                      <option value="0">Disabled</option>
                    </Select>
                  </label>
                ))}
                <label style={{ display: "flex", flexDirection: "column", gap: 6, flex: 1, minWidth: 160 }}>
                  <span className="eyebrow">Reason</span>
                  <Input value={overrideReason} onChange={(e) => setOverrideReason(e.target.value)} placeholder="e.g. support ticket #123" />
                </label>
                <Button onClick={applyOverride} disabled={busy || !overrideKey}>
                  {busy ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Apply override
                </Button>
              </HospitalityCard>
              <p style={{ color: "var(--ink-4)", fontSize: 12, marginTop: 8 }}>
                Overrides take precedence over the plan. Source shows <span className="mono">org_assignment</span> /{" "}
                <span className="mono">restaurant_bridge</span> / <span className="mono">free_default</span>.
              </p>
            </Section>
          )}
        </>
      )}
    </div>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <span className="eyebrow">{label}</span>
      <span style={{ fontSize: 14, color: "var(--ink-1)", textTransform: "capitalize" }}>{value}</span>
    </div>
  )
}

// Minimal themed select (the design system has no Select primitive in components/ui).
function Select({ value, onChange, children }: { value: string | number; onChange: (v: string) => void; children: React.ReactNode }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      style={{
        height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
        background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit",
      }}
    >
      {children}
    </select>
  )
}
