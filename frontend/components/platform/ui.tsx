"use client"

import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog"
import { Loader2 } from "lucide-react"
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from "recharts"
import type { OrgStatus } from "@/types/platform"

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: string; actions?: React.ReactNode }) {
  return (
    <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, marginBottom: 24 }}>
      <div>
        <h1 className="serif" style={{ fontSize: 26, fontWeight: 500, margin: 0 }}>{title}</h1>
        {subtitle && <p style={{ color: "var(--ink-3)", marginTop: 6, fontSize: 14 }}>{subtitle}</p>}
      </div>
      {actions && <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>{actions}</div>}
    </div>
  )
}

export function StatCard({ label, value, hint }: { label: string; value: React.ReactNode; hint?: string }) {
  return (
    <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", flexDirection: "column", gap: 6 }}>
      <span className="eyebrow">{label}</span>
      <span className="serif" style={{ fontSize: 28, fontWeight: 500, color: "var(--ink-1)", lineHeight: 1 }}>{value}</span>
      {hint && <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{hint}</span>}
    </HospitalityCard>
  )
}

export function Section({ title, children, actions }: { title: string; children: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <section style={{ marginTop: 28 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
        <h2 className="eyebrow" style={{ fontSize: 11 }}>{title}</h2>
        {actions}
      </div>
      {children}
    </section>
  )
}

const STATUS_TONE: Record<OrgStatus, { bg: string; fg: string }> = {
  active: { bg: "var(--ok-soft)", fg: "var(--ok)" },
  suspended: { bg: "var(--alert-soft)", fg: "var(--alert)" },
  archived: { bg: "var(--line-2)", fg: "var(--ink-3)" },
}

export function PlatformStatusBadge({ status }: { status: OrgStatus | string }) {
  const tone = STATUS_TONE[status as OrgStatus] ?? { bg: "var(--line-2)", fg: "var(--ink-3)" }
  return (
    <span
      style={{
        display: "inline-flex", alignItems: "center", gap: 5,
        fontSize: 11, fontWeight: 500, textTransform: "uppercase", letterSpacing: "0.03em",
        padding: "3px 8px 3px 7px", borderRadius: "var(--rad-pill)",
        background: tone.bg, color: tone.fg, lineHeight: 1.3, whiteSpace: "nowrap",
      }}
    >
      <span style={{ width: 5, height: 5, borderRadius: "50%", background: tone.fg, flexShrink: 0 }} />
      {status}
    </span>
  )
}

export function ConfirmDialog({
  open, onOpenChange, title, description, confirmLabel, destructive, loading, onConfirm,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  title: string
  description?: string
  confirmLabel: string
  destructive?: boolean
  loading?: boolean
  onConfirm: () => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
          <Button variant={destructive ? "destructive" : "default"} disabled={loading} onClick={onConfirm}>
            {loading ? <Loader2 className="animate-spin" style={{ width: 14, height: 14 }} /> : null}
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function PlatformLoading() {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", padding: "64px 0" }}>
      <Loader2 className="animate-spin" style={{ width: 22, height: 22, color: "var(--ink-3)" }} />
    </div>
  )
}

function formatDay(dateStr: string): string {
  if (!dateStr) return ""
  try {
    return new Date(dateStr).toLocaleDateString("en", { month: "short", day: "numeric" })
  } catch {
    return dateStr
  }
}

// Generic per-day line chart for count or revenue series.
export function DaySeriesChart({
  data, valueKey, label, money,
}: {
  data: { day: string; value: number }[]
  valueKey?: string
  label: string
  money?: boolean
}) {
  void valueKey
  if (data.length === 0) {
    return <p style={{ color: "var(--ink-3)", fontSize: 13, textAlign: "center", padding: "24px 0" }}>No data in this period.</p>
  }
  const series = data.map((d) => ({ label: formatDay(d.day), value: d.value }))
  return (
    <ResponsiveContainer width="100%" height={180}>
      <LineChart data={series} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--line-1)" vertical={false} />
        <XAxis dataKey="label" tick={{ fontSize: 9, fill: "var(--ink-4)" }} axisLine={false} tickLine={false} />
        <YAxis tick={{ fontSize: 9, fill: "var(--ink-4)" }} axisLine={false} tickLine={false} allowDecimals={false} />
        <Tooltip
          contentStyle={{
            background: "var(--bg-elev-3)", border: "1px solid var(--line-2)",
            borderRadius: "var(--rad-md)", fontSize: 12, color: "var(--ink-1)", boxShadow: "var(--shadow-2)",
          }}
          formatter={(value) => [money ? `₹${Number(value).toFixed(2)}` : Number(value), label]}
          cursor={{ stroke: "var(--accent)", strokeWidth: 1, strokeDasharray: "4 2" }}
        />
        <Line type="monotone" dataKey="value" stroke="var(--accent)" strokeWidth={2} dot={false} activeDot={{ r: 4, fill: "var(--accent)", strokeWidth: 0 }} />
      </LineChart>
    </ResponsiveContainer>
  )
}

export function sumCounts(rows: { count: number }[]): number {
  return rows.reduce((acc, r) => acc + r.count, 0)
}
