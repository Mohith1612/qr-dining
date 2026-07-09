"use client"

import { useCallback, useEffect, useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { UsageReport, RevenueReport, HealthReport } from "@/types/platform"
import type { AnalyticsPeriod } from "@/lib/api/analytics"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { PageHeader, StatCard, Section, PlatformLoading, DaySeriesChart, sumCounts } from "@/components/platform/ui"
import { toast } from "sonner"

export default function PlatformOverviewPage() {
  const token = usePlatformStore((s) => s.token)
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [revenue, setRevenue] = useState<RevenueReport | null>(null)
  const [health, setHealth] = useState<HealthReport | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    try {
      const [u, r, h] = await Promise.all([
        platformApi.getUsage(period, undefined, token),
        platformApi.getRevenue(period, undefined, token),
        platformApi.getHealth(period, undefined, token),
      ])
      setUsage(u); setRevenue(r); setHealth(h)
    } catch {
      toast.error("Couldn't load platform metrics.")
    } finally {
      setLoading(false)
    }
  }, [token, period])

  useEffect(() => { load() }, [load])

  return (
    <div>
      <PageHeader
        title="Overview"
        subtitle="Platform-wide operational visibility"
        actions={<PeriodSelector value={period} onChange={setPeriod} />}
      />

      {loading ? <PlatformLoading /> : (
        <>
          <Section title="Usage">
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12 }}>
              <StatCard label="Sessions" value={usage ? sumCounts(usage.sessions_per_day) : 0} hint="in period" />
              <StatCard label="Orders" value={usage ? sumCounts(usage.orders_per_day) : 0} hint="in period" />
              <StatCard label="Payments" value={usage ? sumCounts(usage.payments_per_day) : 0} hint="completed" />
              <StatCard label="Participant joins" value={usage ? sumCounts(usage.participant_joins_per_day) : 0} hint="in period" />
              <StatCard label="Active branches" value={usage?.active_branches ?? 0} hint="with sessions" />
              <StatCard label="Active diners" value={usage?.active_diners ?? 0} hint="right now" />
            </div>
          </Section>

          <Section title="Revenue">
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12 }}>
              <StatCard label="GMV" value={`₹${revenue?.gmv ?? "0"}`} hint="completed payments" />
              <HospitalityCard elev={1} style={{ padding: "16px 18px", gridColumn: "span 2", minWidth: 0 }}>
                <span className="eyebrow">Revenue / day</span>
                <div style={{ marginTop: 8 }}>
                  <DaySeriesChart
                    data={(revenue?.revenue_per_day ?? []).map((d) => ({ day: d.day, value: parseFloat(d.revenue) }))}
                    label="Revenue" money
                  />
                </div>
              </HospitalityCard>
            </div>
            <div style={{ marginTop: 12 }}>
              <HospitalityCard elev={1} style={{ padding: "16px 18px" }}>
                <span className="eyebrow">Top organizations</span>
                <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 8 }}>
                  {(revenue?.revenue_by_org ?? []).slice(0, 5).map((o) => (
                    <div key={o.organization_id} style={{ display: "flex", justifyContent: "space-between", fontSize: 13 }}>
                      <span className="mono" style={{ color: "var(--ink-2)" }}>{o.organization_code}</span>
                      <span style={{ color: "var(--ink-1)", fontWeight: 600 }}>₹{o.revenue}</span>
                    </div>
                  ))}
                  {(revenue?.revenue_by_org ?? []).length === 0 && (
                    <p style={{ color: "var(--ink-3)", fontSize: 13 }}>No revenue in this period.</p>
                  )}
                </div>
              </HospitalityCard>
            </div>
          </Section>

          <Section title="Operational health">
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 }}>
              <StatCard label="Authz denials" value={health ? sumCounts(health.authz_denials_per_day) : 0} hint="in period" />
              <StatCard label="Webhook failures" value={health ? sumCounts(health.webhook_failures_per_day) : 0} hint="in period" />
            </div>
            <p style={{ color: "var(--ink-4)", fontSize: 12, marginTop: 10 }}>
              QR scans, websocket reconnects, and worker failures are tracked in Prometheus, not here.
            </p>
          </Section>
        </>
      )}
    </div>
  )
}
