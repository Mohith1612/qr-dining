"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams } from "next/navigation"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { hasPlatformRole } from "@/lib/platform-rbac"
import type { PlatformPlan, EntitlementCatalogEntry, PlanEntitlement } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { PageHeader, Section, PlatformLoading } from "@/components/platform/ui"
import { ArrowLeft, Loader2 } from "lucide-react"
import { toast } from "sonner"

type Draft = Record<string, { enabled: boolean; limit: string }>

export default function PlanDetailPage() {
  const params = useParams<{ planId: string }>()
  const planId = Number(params.planId)
  const { token, roles } = usePlatformStore()
  const canManage = hasPlatformRole(roles)

  const [plan, setPlan] = useState<PlatformPlan | null>(null)
  const [catalog, setCatalog] = useState<EntitlementCatalogEntry[]>([])
  const [name, setName] = useState("")
  const [price, setPrice] = useState("")
  const [draft, setDraft] = useState<Draft>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    if (!token || !planId) return
    setLoading(true)
    try {
      const [{ plans }, { entitlements }] = await Promise.all([
        platformApi.listPlans(token),
        platformApi.listEntitlementCatalog(token),
      ])
      const p = plans.find((x) => x.id === planId) ?? null
      setPlan(p)
      setCatalog(entitlements)
      if (p) {
        setName(p.name)
        setPrice(p.price_monthly)
        const d: Draft = {}
        for (const c of entitlements) {
          const existing = p.entitlements.find((e) => e.key === c.key)
          d[c.key] = {
            enabled: existing?.enabled ?? false,
            limit: existing?.limit_value == null ? "" : String(existing.limit_value),
          }
        }
        setDraft(d)
      }
    } catch {
      toast.error("Couldn't load plan.")
    } finally {
      setLoading(false)
    }
  }, [token, planId])

  useEffect(() => { load() }, [load])

  async function save() {
    if (!token || !plan) return
    setSaving(true)
    try {
      await platformApi.updatePlan(plan.id, { name: name.trim(), price_monthly: price.trim() }, token)
      const entitlements: PlanEntitlement[] = catalog
        .filter((c) => {
          const d = draft[c.key]
          // include capability rows that are enabled, and limit rows that have a value or are enabled
          return c.kind === "capability" ? d.enabled : (d.enabled || d.limit !== "")
        })
        .map((c) => {
          const d = draft[c.key]
          return {
            key: c.key,
            enabled: c.kind === "capability" ? d.enabled : true,
            limit_value: c.kind === "limit" && d.limit !== "" ? Number(d.limit) : null,
          }
        })
      await platformApi.setPlanEntitlements(plan.id, entitlements, token)
      toast.success("Plan saved.")
      await load()
    } catch {
      toast.error("Couldn't save plan.")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <PlatformLoading />
  if (!plan) return <p style={{ color: "var(--ink-3)" }}>Plan not found.</p>

  const capabilities = catalog.filter((c) => c.kind === "capability")
  const limits = catalog.filter((c) => c.kind === "limit")

  return (
    <div>
      <Link href="/platform/plans" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--ink-3)", fontSize: 13, textDecoration: "none", marginBottom: 14 }}>
        <ArrowLeft size={14} aria-hidden /> Plans
      </Link>
      <PageHeader
        title={plan.name}
        subtitle={`${plan.tier} tier`}
        actions={canManage ? (
          <Button onClick={save} disabled={saving}>
            {saving ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null} Save changes
          </Button>
        ) : undefined}
      />

      <Section title="Plan details">
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", gap: 16, flexWrap: "wrap" }}>
          <label style={{ display: "flex", flexDirection: "column", gap: 6, flex: 1, minWidth: 180 }}>
            <span className="eyebrow">Name</span>
            <Input value={name} onChange={(e) => setName(e.target.value)} disabled={!canManage} />
          </label>
          <label style={{ display: "flex", flexDirection: "column", gap: 6, width: 160 }}>
            <span className="eyebrow">Price / month</span>
            <Input value={price} onChange={(e) => setPrice(e.target.value)} disabled={!canManage} inputMode="decimal" />
          </label>
        </HospitalityCard>
      </Section>

      <Section title="Capabilities">
        <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
          {capabilities.map((c) => (
            <label key={c.key} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderBottom: "1px solid var(--line-1)" }}>
              <div>
                <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{c.key}</span>
                <p style={{ fontSize: 12, color: "var(--ink-4)", margin: "2px 0 0" }}>{c.description}</p>
              </div>
              <input
                type="checkbox"
                checked={draft[c.key]?.enabled ?? false}
                disabled={!canManage}
                onChange={(e) => setDraft((d) => ({ ...d, [c.key]: { ...d[c.key], enabled: e.target.checked } }))}
                style={{ width: 18, height: 18, accentColor: "var(--accent)" }}
              />
            </label>
          ))}
        </HospitalityCard>
      </Section>

      <Section title="Limits (blank = unlimited)">
        <HospitalityCard elev={1} style={{ padding: "8px 0" }}>
          {limits.map((c) => (
            <div key={c.key} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 18px", borderBottom: "1px solid var(--line-1)" }}>
              <div>
                <span className="mono" style={{ fontSize: 13, color: "var(--ink-1)" }}>{c.key}</span>
                <p style={{ fontSize: 12, color: "var(--ink-4)", margin: "2px 0 0" }}>{c.description}</p>
              </div>
              <Input
                value={draft[c.key]?.limit ?? ""}
                disabled={!canManage}
                onChange={(e) => setDraft((d) => ({ ...d, [c.key]: { ...d[c.key], limit: e.target.value.replace(/[^0-9]/g, "") } }))}
                placeholder="∞"
                inputMode="numeric"
                style={{ width: 100 }}
              />
            </div>
          ))}
        </HospitalityCard>
      </Section>
    </div>
  )
}
