"use client"

import { useCallback, useEffect, useState } from "react"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import type { Organization, UsageReport, RevenueReport, HealthReport } from "@/types/platform"
import type { AnalyticsPeriod } from "@/lib/api/analytics"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { PageHeader, StatCard, PlatformLoading, DaySeriesChart } from "@/components/platform/ui"
import { toast } from "sonner"

export default function PlatformAnalyticsPage() {
  const token = usePlatformStore((s) => s.token)
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [orgFilter, setOrgFilter] = useState<number | undefined>(undefined)
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [tab, setTab] = useState("usage")

  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [revenue, setRevenue] = useState<RevenueReport | null>(null)
  const [health, setHealth] = useState<HealthReport | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!token) return
    platformApi.listOrganizations(token)
      .then((r) => setOrgs(r.organizations))
      .catch(() => {})
  }, [token])

  const load = useCallback(async () => {
    if (!token) return
    setLoading(true)
    try {
      const [u, r, h] = await Promise.all([
        platformApi.getUsage(period, orgFilter, token),
        platformApi.getRevenue(period, orgFilter, token),
        platformApi.getHealth(period, orgFilter, token),
      ])
      setUsage(u); setRevenue(r); setHealth(h)
    } catch {
      toast.error("Couldn't load analytics.")
    } finally {
      setLoading(false)
    }
  }, [token, period, orgFilter])

  useEffect(() => { load() }, [load])

  return (
    <div>
      <PageHeader
        title="Analytics"
        subtitle="Cross-tenant usage, revenue, and operational health"
        actions={<PeriodSelector value={period} onChange={setPeriod} />}
      />

      <label style={{ display: "flex", flexDirection: "column", gap: 6, maxWidth: 320, marginBottom: 20 }}>
        <span className="eyebrow">Organization filter</span>
        <select
          value={orgFilter ?? ""}
          onChange={(e) => setOrgFilter(e.target.value ? Number(e.target.value) : undefined)}
          style={{ height: 32, borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)", background: "var(--bg-elev-1)", color: "var(--ink-1)", padding: "0 10px", fontSize: 14, fontFamily: "inherit" }}
        >
          <option value="">All organizations</option>
          {orgs.map((o) => <option key={o.id} value={o.id}>{o.name} ({o.code})</option>)}
        </select>
      </label>

      <Tabs value={tab} onValueChange={(v) => setTab(v as string)}>
        <TabsList>
          <TabsTrigger value="usage">Usage</TabsTrigger>
          <TabsTrigger value="revenue">Revenue</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>

        {loading ? <PlatformLoading /> : (
          <>
            <TabsContent value="usage">
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12, marginTop: 12 }}>
                <StatCard label="Active branches" value={usage?.active_branches ?? 0} />
                <StatCard label="Active diners" value={usage?.active_diners ?? 0} />
              </div>
              <ChartCard title="Sessions / day" data={(usage?.sessions_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Sessions" />
              <ChartCard title="Orders / day" data={(usage?.orders_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Orders" />
              <ChartCard title="Payments / day" data={(usage?.payments_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Payments" />
              <ChartCard title="Participant joins / day" data={(usage?.participant_joins_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Joins" />
            </TabsContent>

            <TabsContent value="revenue">
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12, marginTop: 12 }}>
                <StatCard label="GMV" value={`₹${revenue?.gmv ?? "0"}`} hint="completed payments" />
              </div>
              <ChartCard title="Revenue / day" data={(revenue?.revenue_per_day ?? []).map((d) => ({ day: d.day, value: parseFloat(d.revenue) }))} label="Revenue" money />
              <RevenueTable
                title="Revenue by branch"
                rows={(revenue?.revenue_by_branch ?? []).map((b) => ({ key: String(b.branch_id), label: `${b.branch_name} (${b.branch_code})`, count: b.count, revenue: b.revenue }))}
              />
              <RevenueTable
                title="Revenue by organization"
                rows={(revenue?.revenue_by_org ?? []).map((o) => ({ key: String(o.organization_id), label: o.organization_code, count: o.count, revenue: o.revenue }))}
              />
            </TabsContent>

            <TabsContent value="health">
              <ChartCard title="Authz denials / day" data={(health?.authz_denials_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Denials" />
              <ChartCard title="Webhook failures / day" data={(health?.webhook_failures_per_day ?? []).map((d) => ({ day: d.day, value: d.count }))} label="Failures" />
              <p style={{ color: "var(--ink-4)", fontSize: 12, marginTop: 12 }}>
                QR scans, websocket reconnects, worker failures, and audit-write failures are Prometheus-only and not shown here.
              </p>
            </TabsContent>
          </>
        )}
      </Tabs>
    </div>
  )
}

function ChartCard({ title, data, label, money }: { title: string; data: { day: string; value: number }[]; label: string; money?: boolean }) {
  return (
    <HospitalityCard elev={1} style={{ padding: "16px 18px", marginTop: 12 }}>
      <span className="eyebrow">{title}</span>
      <div style={{ marginTop: 8 }}>
        <DaySeriesChart data={data} label={label} money={money} />
      </div>
    </HospitalityCard>
  )
}

function RevenueTable({ title, rows }: { title: string; rows: { key: string; label: string; count: number; revenue: string }[] }) {
  return (
    <HospitalityCard elev={1} style={{ padding: "16px 18px", marginTop: 12 }}>
      <span className="eyebrow">{title}</span>
      <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 8 }}>
        {rows.length === 0 ? <p style={{ color: "var(--ink-3)", fontSize: 13 }}>No data in this period.</p> :
          rows.map((r) => (
            <div key={r.key} style={{ display: "flex", justifyContent: "space-between", fontSize: 13 }}>
              <span style={{ color: "var(--ink-2)" }}>{r.label}</span>
              <span style={{ color: "var(--ink-1)" }}><span style={{ color: "var(--ink-4)" }}>{r.count}× </span><strong>₹{r.revenue}</strong></span>
            </div>
          ))}
      </div>
    </HospitalityCard>
  )
}
