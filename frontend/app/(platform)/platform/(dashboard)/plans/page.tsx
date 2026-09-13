"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { PlatformPlan } from "@/types/platform"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PageHeader, PlatformLoading } from "@/components/platform/ui"
import { ChevronRight } from "lucide-react"
import { toast } from "sonner"

function planCapabilities(plan: PlatformPlan): string[] {
  return plan.entitlements.filter((e) => e.enabled && !e.key.startsWith("limit.")).map((e) => e.key)
}
function planLimits(plan: PlatformPlan): { key: string; value: number | null }[] {
  return plan.entitlements.filter((e) => e.key.startsWith("limit.")).map((e) => ({ key: e.key, value: e.limit_value }))
}

export default function PlansPage() {
  const token = usePlatformStore((s) => s.token)
  const [plans, setPlans] = useState<PlatformPlan[]>([])
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    try {
      const { plans } = await platformApi.listPlans(token)
      setPlans(plans)
    } catch {
      toast.error("Couldn't load plans.")
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => { load() }, [load])

  return (
    <div>
      <PageHeader title="Plans" subtitle="Subscription tiers and their entitlement defaults" />
      {loading ? <PlatformLoading /> : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
          {plans.map((p) => {
            const caps = planCapabilities(p)
            const limits = planLimits(p)
            return (
              <Link key={p.id} href={`/platform/plans/${p.id}`} style={{ textDecoration: "none" }}>
                <HospitalityCard elev={1} press style={{ padding: "18px", display: "flex", flexDirection: "column", gap: 12, height: "100%" }}>
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <div>
                      <span style={{ fontSize: 16, fontWeight: 600, color: "var(--ink-1)" }}>{p.name}</span>
                      <span className="mono" style={{ fontSize: 11, color: "var(--ink-4)", marginLeft: 8 }}>{p.tier}</span>
                    </div>
                    <ChevronRight size={16} style={{ color: "var(--ink-4)" }} aria-hidden />
                  </div>
                  <span className="serif" style={{ fontSize: 22, color: "var(--ink-1)" }}>₹{p.price_monthly}<span style={{ fontSize: 12, color: "var(--ink-4)" }}> /mo</span></span>
                  <div>
                    <span className="eyebrow">Capabilities</span>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 5, marginTop: 6 }}>
                      {caps.length === 0 ? <span style={{ fontSize: 12, color: "var(--ink-3)" }}>None</span> :
                        caps.map((c) => (
                          <span key={c} className="mono" style={{ fontSize: 10.5, padding: "2px 7px", borderRadius: "var(--rad-pill)", background: "var(--accent-soft)", color: "var(--accent)" }}>{c}</span>
                        ))}
                    </div>
                  </div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 12 }}>
                    {limits.map((l) => (
                      <span key={l.key} style={{ fontSize: 12, color: "var(--ink-2)" }}>
                        <span className="mono" style={{ color: "var(--ink-4)" }}>{l.key.replace("limit.", "")}</span>{" "}
                        <strong style={{ color: "var(--ink-1)" }}>{l.value == null ? "∞" : l.value}</strong>
                      </span>
                    ))}
                  </div>
                </HospitalityCard>
              </Link>
            )
          })}
        </div>
      )}
    </div>
  )
}
