import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { ChefHat } from "lucide-react"
import type {
  WaiterPerformanceRow,
  KitchenPerformanceRow,
  StaffDailyActivityRow,
} from "@/lib/api/analytics"

export function fmtDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return "—"
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}

interface StaffPerformanceTablesProps {
  waiters: WaiterPerformanceRow[]
  kitchen: KitchenPerformanceRow[]
  summary: StaffDailyActivityRow[]
}

// Presentational staff-performance tables (front of house / kitchen / daily activity),
// shared by the manager admin tab and the platform analytics page.
export function StaffPerformanceTables({ waiters, kitchen, summary }: StaffPerformanceTablesProps) {
  return (
    <div className="flex flex-col gap-4">
      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Front of house</SectionHeader>
        {waiters.length === 0 ? (
          <p className="text-sm opacity-60">No staff activity in this period.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left opacity-60">
                  <th className="py-2 pr-3 font-medium">Staff</th>
                  <th className="py-2 pr-3 font-medium">Sessions</th>
                  <th className="py-2 pr-3 font-medium">Tables</th>
                  <th className="py-2 pr-3 font-medium">Served</th>
                  <th className="py-2 pr-3 font-medium">Assists</th>
                  <th className="py-2 pr-3 font-medium">Avg response</th>
                  <th className="py-2 pr-3 font-medium">Settled</th>
                  <th className="py-2 pr-3 font-medium">Avg settle</th>
                  <th className="py-2 pr-3 font-medium">Logins</th>
                  <th className="py-2 font-medium">Active</th>
                </tr>
              </thead>
              <tbody>
                {waiters.map((w) => (
                  <tr key={w.staff_id} className="border-t border-white/5">
                    <td className="py-2 pr-3">
                      {w.staff_name}
                      <span className="ml-2 text-xs opacity-50">{w.staff_role}</span>
                    </td>
                    <td className="py-2 pr-3">{w.sessions_handled}</td>
                    <td className="py-2 pr-3">{w.tables_served}</td>
                    <td className="py-2 pr-3">{w.orders_served}</td>
                    <td className="py-2 pr-3">
                      {w.assistance_accepted}/{w.assistance_resolved}
                    </td>
                    <td className="py-2 pr-3">{fmtDuration(w.avg_response_seconds)}</td>
                    <td className="py-2 pr-3">{w.payments_settled}</td>
                    <td className="py-2 pr-3">{fmtDuration(w.avg_settlement_seconds)}</td>
                    <td className="py-2 pr-3">{w.login_count}</td>
                    <td className="py-2">{fmtDuration(w.active_seconds)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </HospitalityCard>

      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Kitchen</SectionHeader>
        {kitchen.length === 0 ? (
          <p className="text-sm opacity-60">No kitchen activity in this period.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
            {kitchen.map((k) => (
              <div key={k.staff_id} className="rounded-lg border border-white/10 p-4">
                <div className="flex items-center gap-2 mb-2">
                  <ChefHat size={16} className="opacity-60" />
                  <span className="font-medium text-sm">{k.staff_name}</span>
                </div>
                <dl className="text-sm flex flex-col gap-1">
                  <div className="flex justify-between">
                    <dt className="opacity-60">Orders completed</dt>
                    <dd>{k.orders_completed}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="opacity-60">Avg prep time</dt>
                    <dd>{fmtDuration(k.avg_prep_seconds)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="opacity-60">Peak / hour</dt>
                    <dd>{k.peak_orders_per_hour}</dd>
                  </div>
                </dl>
              </div>
            ))}
          </div>
        )}
      </HospitalityCard>

      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Daily activity</SectionHeader>
        {summary.length === 0 ? (
          <p className="text-sm opacity-60">No activity in this period.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left opacity-60">
                  <th className="py-2 pr-3 font-medium">Day</th>
                  <th className="py-2 pr-3 font-medium">Staff</th>
                  <th className="py-2 pr-3 font-medium">Actions</th>
                  <th className="py-2 pr-3 font-medium">Logins</th>
                  <th className="py-2 font-medium">Active</th>
                </tr>
              </thead>
              <tbody>
                {summary.slice(0, 50).map((r) => (
                  <tr key={`${r.day}-${r.staff_id}`} className="border-t border-white/5">
                    <td className="py-2 pr-3">{r.day}</td>
                    <td className="py-2 pr-3">
                      {r.staff_name}
                      <span className="ml-2 text-xs opacity-50">{r.staff_role}</span>
                    </td>
                    <td className="py-2 pr-3">{r.event_count}</td>
                    <td className="py-2 pr-3">{r.login_count}</td>
                    <td className="py-2">{fmtDuration(r.active_seconds)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </HospitalityCard>
    </div>
  )
}
