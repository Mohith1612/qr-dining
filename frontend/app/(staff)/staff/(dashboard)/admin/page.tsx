"use client"

import { useEffect, useState, useCallback } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { menuApi } from "@/lib/api/menu"
import { analyticsApi, type AnalyticsPeriod, type TopItem, type BusyHour, type OrderVolumeDay } from "@/lib/api/analytics"
import { plansApi, type Subscription, type Plan } from "@/lib/api/plans"
import { ApiError } from "@/lib/api/client"
import { useTenant } from "@/providers/TenantProvider"
import { PeriodSelector } from "@/components/shared/PeriodSelector"
import { TopItemsList } from "@/components/analytics/TopItemsList"
import { BusyHoursChart } from "@/components/analytics/BusyHoursChart"
import { OrderVolumeChart } from "@/components/analytics/OrderVolumeChart"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, StaffRole, Table } from "@/types/api"
import { tablesApi } from "@/lib/api/tables"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { BottomSheet } from "@/components/shared/BottomSheet"

// ─── Sessions Tab ───────────────────────────────────────────────────────────

function SessionsTab() {
  const { branchId, token } = useStaffStore()
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  const fetchSessions = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await staffApi.getActiveSessions(branchId, token)
      setSessions(data)
    } catch {
      toast.error("Couldn't load sessions.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
          {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={fetchSessions}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6,
            fontSize: 12, color: "var(--ink-3)",
            padding: "6px 10px", borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
          }}
          aria-label="Refresh sessions"
        >
          <RefreshCw size={12} aria-hidden />
          Refresh
        </button>
      </div>

      {sessions.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No active sessions"
          description="Active sessions will appear here."
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {sessions.map((s) => (
            <HospitalityCard
              key={s.id}
              elev={1}
              style={{ padding: "14px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}
            >
              <div style={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 3 }}>
                <p className="mono" style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-1)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  #{s.id.slice(0, 8)}
                </p>
                <p style={{ fontSize: 12, color: "var(--ink-3)" }}>
                  {s.table_identifier ?? `Table ${s.table_id}`} · {relativeTime(s.created_at)}
                </p>
              </div>
              <span
                style={{
                  fontSize: 11, fontWeight: 500,
                  padding: "3px 10px", borderRadius: "var(--rad-pill)",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-3)", flexShrink: 0,
                  letterSpacing: "0.03em", textTransform: "uppercase",
                }}
              >
                {s.status}
              </span>
            </HospitalityCard>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Menu Tab ────────────────────────────────────────────────────────────────

function MenuTab() {
  const { branchId, token, role } = useStaffStore()
  const [categories, setCategories] = useState<MenuCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [toggling, setToggling] = useState<number | null>(null)
  const [togglingFeatured, setTogglingFeatured] = useState<number | null>(null)

  useEffect(() => {
    if (!branchId) return
    menuApi.getMenu(branchId).then(({ categories }) => setCategories(categories)).catch(() => {
      toast.error("Couldn't load menu.")
    }).finally(() => setLoading(false))
  }, [branchId])

  async function handleToggle(itemId: number, current: boolean) {
    if (!branchId || !token) return
    setToggling(itemId)
    const next = !current
    setCategories((prev) =>
      prev.map((cat) => ({
        ...cat,
        items: cat.items.map((item) =>
          item.id === itemId ? { ...item, is_available: next } : item
        ),
      }))
    )
    try {
      await staffApi.toggleAvailability(itemId, branchId, next, token)
    } catch {
      setCategories((prev) =>
        prev.map((cat) => ({
          ...cat,
          items: cat.items.map((item) =>
            item.id === itemId ? { ...item, is_available: current } : item
          ),
        }))
      )
      toast.error("Couldn't update availability.")
    } finally {
      setToggling(null)
    }
  }

  async function handleToggleFeatured(itemId: number, currentFeatured: boolean, sortOrder: number) {
    if (!branchId || !token) return
    setTogglingFeatured(itemId)
    const next = !currentFeatured
    setCategories((prev) =>
      prev.map((cat) => ({
        ...cat,
        items: cat.items.map((item) =>
          item.id === itemId ? { ...item, is_featured: next } : item
        ),
      }))
    )
    try {
      await staffApi.toggleFeatured(itemId, branchId, next, sortOrder, token)
    } catch {
      setCategories((prev) =>
        prev.map((cat) => ({
          ...cat,
          items: cat.items.map((item) =>
            item.id === itemId ? { ...item, is_featured: currentFeatured } : item
          ),
        }))
      )
      toast.error("Couldn't update featured status.")
    } finally {
      setTogglingFeatured(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
      {categories.map((cat) => (
        <section key={cat.id}>
          <p className="eyebrow" style={{ marginBottom: 10 }}>{cat.name}</p>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {cat.items.map((item) => (
              <HospitalityCard
                key={item.id}
                elev={1}
                style={{
                  padding: "12px 16px",
                  display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12,
                  opacity: item.is_available ? 1 : 0.55,
                }}
              >
                <div style={{ minWidth: 0 }}>
                  <p style={{ fontSize: 14, fontWeight: 500, color: "var(--ink-1)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {item.name}
                  </p>
                  <p style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 1 }}>
                    {formatCurrency(item.price)}
                  </p>
                </div>
                {role === 'owner' && (
                  <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
                    <button
                      onClick={() => handleToggleFeatured(item.id, item.is_featured ?? false, item.featured_sort_order ?? 0)}
                      disabled={togglingFeatured === item.id}
                      className="press"
                      style={{
                        fontSize: 12, fontWeight: 600,
                        padding: "6px 14px",
                        borderRadius: "var(--rad-pill)",
                        border: "1px solid",
                        minHeight: 36,
                        display: "flex", alignItems: "center", justifyContent: "center",
                        borderColor: item.is_featured ? "var(--accent)" : "var(--line-2)",
                        background: item.is_featured ? "var(--accent-soft)" : "var(--bg-elev-2)",
                        color: item.is_featured ? "var(--accent)" : "var(--ink-3)",
                        transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
                        cursor: togglingFeatured === item.id ? "not-allowed" : "pointer",
                      }}
                      aria-label={item.is_featured ? "Remove from featured" : "Add to featured"}
                    >
                      {togglingFeatured === item.id ? (
                        <Loader2 size={13} className="animate-spin" />
                      ) : item.is_featured ? (
                        "Featured"
                      ) : (
                        "Feature"
                      )}
                    </button>
                    <button
                      onClick={() => handleToggle(item.id, item.is_available)}
                      disabled={toggling === item.id}
                      className="press"
                      style={{
                        fontSize: 12, fontWeight: 600,
                        padding: "6px 14px",
                        borderRadius: "var(--rad-pill)",
                        border: "none",
                        minHeight: 36, minWidth: 72,
                        display: "flex", alignItems: "center", justifyContent: "center",
                        background: item.is_available ? "var(--ok)" : "var(--bg-elev-3)",
                        color: item.is_available ? "white" : "var(--ink-3)",
                        transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease)",
                        cursor: toggling === item.id ? "not-allowed" : "pointer",
                      }}
                      aria-label={item.is_available ? "Mark unavailable" : "Mark available"}
                    >
                      {toggling === item.id ? (
                        <Loader2 size={13} className="animate-spin" />
                      ) : item.is_available ? (
                        "Available"
                      ) : (
                        "Off"
                      )}
                    </button>
                  </div>
                )}
              </HospitalityCard>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}

// ─── Staff Tab ───────────────────────────────────────────────────────────────

const STAFF_ROLES: { value: StaffRole; label: string }[] = [
  { value: "waiter",  label: "Waiter"  },
  { value: "kitchen", label: "Kitchen" },
  { value: "manager", label: "Manager" },
]

function StaffTab() {
  const { branchId, token, role } = useStaffStore()
  const [name, setName] = useState("")
  const [selectedRole, setSelectedRole] = useState<StaffRole>("waiter")
  const [pin, setPin] = useState("")
  const [creating, setCreating] = useState(false)

  const canManage = role === "owner" || role === "manager"

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!branchId || !token || !name || !pin) return
    setCreating(true)
    try {
      await staffApi.createStaff(branchId, name, selectedRole, pin, token)
      toast.success(`${name} added as ${selectedRole}`)
      setName("")
      setPin("")
    } catch {
      toast.error("Couldn't create staff account.")
    } finally {
      setCreating(false)
    }
  }

  if (!canManage) {
    return (
      <div style={{ padding: "48px 0", textAlign: "center", color: "var(--ink-3)" }}>
        <p style={{ fontSize: 14 }}>Only owners and managers can manage staff accounts.</p>
      </div>
    )
  }

  return (
    <div>
      <HospitalityCard elev={2} style={{ padding: 20 }}>
        <p className="serif" style={{ fontSize: 18, fontWeight: 500, color: "var(--ink-1)", marginBottom: 16 }}>
          Add staff member
        </p>

        <form onSubmit={handleCreate} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Staff name"
              required
              style={{
                height: 44,
                borderRadius: "var(--rad-lg)",
                background: "var(--bg-base)",
                borderColor: "var(--line-2)",
                color: "var(--ink-1)",
              }}
            />
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              Role
            </label>
            <div style={{ display: "flex", gap: 6 }}>
              {STAFF_ROLES.map(({ value, label }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setSelectedRole(value)}
                  className="press"
                  style={{
                    flex: 1, padding: "10px 8px",
                    borderRadius: "var(--rad-md)",
                    fontSize: 13, fontWeight: 500,
                    border: "1px solid",
                    borderColor: selectedRole === value ? "var(--accent)" : "var(--line-2)",
                    background: selectedRole === value ? "var(--accent-soft)" : "var(--bg-base)",
                    color: selectedRole === value ? "var(--accent)" : "var(--ink-3)",
                    minHeight: 44,
                    transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
                  }}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 600, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
              PIN
            </label>
            <Input
              type="password"
              inputMode="numeric"
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              placeholder="4–6 digits"
              maxLength={6}
              required
              style={{
                height: 44,
                borderRadius: "var(--rad-lg)",
                background: "var(--bg-base)",
                borderColor: "var(--line-2)",
                color: "var(--ink-1)",
                letterSpacing: "0.2em",
              }}
            />
          </div>

          <Button
            type="submit"
            disabled={creating || !name || !pin}
            style={{
              width: "100%", height: 44,
              borderRadius: "var(--rad-lg)",
              background: creating ? "var(--bg-elev-3)" : "linear-gradient(180deg, var(--accent-strong), var(--accent))",
              color: creating ? "var(--ink-3)" : "var(--accent-ink)",
              border: "none", fontSize: 14, fontWeight: 600,
            }}
          >
            {creating ? (
              <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Loader2 size={16} className="animate-spin" />
                Creating…
              </span>
            ) : (
              "Create account"
            )}
          </Button>
        </form>
      </HospitalityCard>
    </div>
  )
}

// ─── Stats Tab ───────────────────────────────────────────────────────────────

function StatsTab() {
  const { branchId, token } = useStaffStore()
  const [period, setPeriod] = useState<AnalyticsPeriod>("weekly")
  const [topItems, setTopItems] = useState<TopItem[]>([])
  const [busyHours, setBusyHours] = useState<BusyHour[]>([])
  const [orderVolume, setOrderVolume] = useState<OrderVolumeDay[]>([])
  const [gated, setGated] = useState(false)
  const [loading, setLoading] = useState(true)

  const fetchAnalytics = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    setGated(false)
    try {
      const [topResult, busyResult, volumeResult] = await Promise.all([
        analyticsApi.getTopItems(branchId, period, token),
        analyticsApi.getBusyHours(branchId, period, token),
        analyticsApi.getOrderVolume(branchId, period, token),
      ])
      setTopItems(topResult.items)
      setBusyHours(busyResult.hours)
      setOrderVolume(volumeResult.days)
    } catch (err) {
      if (err instanceof ApiError && err.code === "ANALYTICS_GATED") {
        setGated(true)
      } else {
        toast.error("Couldn't load analytics.")
      }
    } finally {
      setLoading(false)
    }
  }, [branchId, token, period])

  useEffect(() => {
    fetchAnalytics()
  }, [fetchAnalytics])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  if (gated) {
    return (
      <EmptyState
        icon={BarChart2}
        title="Analytics not available"
        description="Analytics is available on Standard and Premium plans."
        action={{ label: "View pricing", onClick: () => window.location.href = "/pricing" }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <PeriodSelector value={period} onChange={setPeriod} />

      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Top items</SectionHeader>
        <TopItemsList items={topItems} />
      </HospitalityCard>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <HospitalityCard elev={1} className="p-5">
          <SectionHeader className="mb-4">Busy hours</SectionHeader>
          <BusyHoursChart hours={busyHours} />
        </HospitalityCard>

        <HospitalityCard elev={1} className="p-5">
          <SectionHeader className="mb-4">Order volume</SectionHeader>
          <OrderVolumeChart days={orderVolume} />
        </HospitalityCard>
      </div>
    </div>
  )
}

// ─── Plan Tab ────────────────────────────────────────────────────────────────

function formatFeatureVal(key: keyof Plan["features_json"], val: number | boolean): string {
  if (typeof val === "boolean") return val ? "✓" : "—"
  return val === -1 ? "Unlimited" : String(val)
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleDateString("en", { month: "short", day: "numeric", year: "numeric" })
  } catch {
    return dateStr
  }
}

const PLAN_FEATURES: { label: string; key: keyof Plan["features_json"] }[] = [
  { label: "Branches",     key: "max_branches"  },
  { label: "Tables",       key: "max_tables"     },
  { label: "Analytics",    key: "analytics"      },
  { label: "Multi-branch", key: "multi_branch"   },
]

function PlanTab() {
  const { token } = useStaffStore()
  const { restaurantId, ready: tenantReady } = useTenant()
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [features, setFeatures] = useState<Plan["features_json"] | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!tenantReady) return
    if (!restaurantId || !token) {
      setLoading(false)
      return
    }
    plansApi
      .getSubscription(restaurantId, token)
      .then((data) => {
        setSubscription(data.subscription)
        setFeatures(data.features)
      })
      .catch(() => toast.error("Couldn't load subscription."))
      .finally(() => setLoading(false))
  }, [restaurantId, token, tenantReady])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin opacity-40" />
      </div>
    )
  }

  if (!restaurantId) {
    return (
      <div className="py-12 text-center">
        <p className="text-[13px] text-[var(--ink-3)]">
          No tenant context. Set{" "}
          <code className="font-mono text-[11px] px-1.5 py-0.5 rounded-[var(--rad-sm)] bg-[var(--bg-elev-2)] border border-[var(--line-2)] text-[var(--ink-2)]">
            NEXT_PUBLIC_TENANT_SLUG
          </code>{" "}
          in your local environment.
        </p>
      </div>
    )
  }

  if (!subscription || !features) {
    return <EmptyState icon={CreditCard} title="No subscription found." />
  }

  const isPaid = subscription.plan_tier !== "free"
  const isActive = subscription.status === "active"
  const isTrial = subscription.status === "trial"

  return (
    <div>
      <HospitalityCard elev={2} className="p-5">
        <div className="flex items-center gap-2 flex-wrap mb-1">
          <span className="serif text-lg font-medium text-[var(--ink-1)]">
            {subscription.plan_name}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest border",
              isPaid
                ? "bg-[var(--accent-soft)] text-[var(--accent)] border-[var(--accent)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)] border-[var(--line-2)]"
            )}
          >
            {subscription.plan_tier}
          </span>
          <span
            className={cn(
              "text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-widest",
              isActive
                ? "bg-[var(--ok-soft)] text-[var(--ok)]"
                : isTrial
                ? "bg-[var(--warn-soft)] text-[var(--warn)]"
                : "bg-[var(--bg-elev-2)] text-[var(--ink-3)]"
            )}
          >
            {subscription.status}
          </span>
        </div>

        {isTrial && subscription.trial_ends_at && (
          <p className="text-[13px] text-[var(--ink-3)] mb-4">
            Trial ends {formatDate(subscription.trial_ends_at)}
          </p>
        )}

        <hr className="rule-strong my-4" />

        <div className="flex flex-col mb-5">
          {PLAN_FEATURES.map(({ label, key }, i) => {
            const val = features[key]
            const inactive = (typeof val === "boolean" && !val) || val === 0
            return (
              <div key={key}>
                {i > 0 && <hr className="rule" />}
                <div className="flex justify-between items-center py-2.5">
                  <span className="text-[13px] text-[var(--ink-3)]">{label}</span>
                  <span className={cn("serif text-[15px] font-medium", inactive ? "text-[var(--ink-4)]" : "text-[var(--ink-1)]")}>
                    {formatFeatureVal(key, val)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>

        <Link
          href="/pricing"
          className="press block text-center py-2.5 rounded-[var(--rad-md)] border border-[var(--line-2)] text-[13px] font-medium text-[var(--ink-2)] no-underline"
        >
          View all plans
        </Link>
      </HospitalityCard>
    </div>
  )
}

// ─── Tables Tab ──────────────────────────────────────────────────────────────

function TableRow({
  table,
  canManage,
  onPrint,
  onRefreshQR,
}: {
  table: Table
  canManage: boolean
  onPrint: (t: Table) => void
  onRefreshQR: (t: Table) => void
}) {
  const isOccupied = table.status === "occupied"

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "10px 14px",
        background: "var(--bg-elev-1)",
        border: "1px solid var(--line-1)",
        borderRadius: "var(--rad-md)",
      }}
    >
      {/* Identifier + capacity */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <span className="serif" style={{ fontSize: 18, color: "var(--ink-1)", fontWeight: 500 }}>
          {table.identifier}
        </span>
        <span style={{ fontSize: 12, color: "var(--ink-3)", marginLeft: 8 }}>
          {table.capacity} seats
        </span>
      </div>

      {/* Status pill */}
      <span
        style={{
          fontSize: 11,
          fontWeight: 600,
          letterSpacing: "0.06em",
          textTransform: "uppercase",
          padding: "2px 8px",
          borderRadius: 99,
          background: isOccupied ? "rgba(251,191,36,0.12)" : "rgba(52,211,153,0.12)",
          color: isOccupied ? "#F59E0B" : "#10B981",
          flexShrink: 0,
        }}
      >
        {isOccupied ? "Occupied" : "Available"}
      </span>

      {/* QR preview */}
      <QRCard table={table} size={48} />

      {/* Actions */}
      <button
        onClick={() => onPrint(table)}
        className="press"
        title="Print QR card"
        style={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          fontSize: 12,
          color: "var(--ink-2)",
          padding: "5px 10px",
          borderRadius: "var(--rad-md)",
          background: "var(--bg-elev-2)",
          border: "1px solid var(--line-1)",
          flexShrink: 0,
        }}
      >
        <Printer size={12} aria-hidden />
        Print
      </button>

      {canManage && (
        <button
          onClick={() => onRefreshQR(table)}
          disabled={isOccupied}
          className="press"
          title={isOccupied ? "Cannot regenerate while table is occupied" : "Regenerate QR token"}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 4,
            fontSize: 12,
            padding: "5px 10px",
            borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)",
            border: "1px solid var(--line-1)",
            flexShrink: 0,
            color: isOccupied ? "var(--ink-4)" : "var(--ink-2)",
            cursor: isOccupied ? "not-allowed" : "pointer",
            opacity: isOccupied ? 0.5 : 1,
          }}
        >
          <RotateCcw size={12} aria-hidden />
          Regen QR
        </button>
      )}
    </div>
  )
}

function TablesTab() {
  const { branchId, token, role } = useStaffStore()
  const { slug } = useTenant()
  const [tables, setTables] = useState<Table[]>([])
  const [loading, setLoading] = useState(true)
  const [createModal, setCreateModal] = useState(false)
  const [creating, setCreating] = useState(false)
  const [identifier, setIdentifier] = useState("")
  const [capacity, setCapacity] = useState(4)
  const [printTable, setPrintTable] = useState<Table | null>(null)

  const canManage = role === "owner" || role === "manager"

  const fetchTables = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await tablesApi.list(branchId, token)
      setTables(data)
    } catch {
      toast.error("Couldn't load tables.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => {
    fetchTables()
  }, [fetchTables])

  async function handleCreate() {
    if (!branchId || !token || !identifier.trim()) return
    setCreating(true)
    try {
      const table = await tablesApi.create(branchId, { identifier: identifier.trim(), capacity }, token)
      setTables(prev => [...prev, table].sort((a, b) => a.identifier.localeCompare(b.identifier)))
      setCreateModal(false)
      setIdentifier("")
      setCapacity(4)
      toast.success(`Table ${table.identifier} created.`)
    } catch {
      toast.error("Failed to create table.")
    } finally {
      setCreating(false)
    }
  }

  async function handleRefreshQR(table: Table) {
    if (!token) return
    if (!confirm(`Regenerate QR for ${table.identifier}? This will invalidate all printed cards.`)) return
    try {
      const updated = await tablesApi.refreshQR(table.id, token)
      setTables(prev => prev.map(t => t.id === updated.id ? updated : t))
      toast.success(`QR token regenerated for ${table.identifier}.`)
    } catch {
      toast.error("Failed to regenerate QR token.")
    }
  }

  function handlePrint(table: Table) {
    setPrintTable(table)
    setTimeout(() => window.print(), 100)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
        <p className="eyebrow">{tables.length} table{tables.length !== 1 ? "s" : ""}</p>
        {canManage && (
          <button
            onClick={() => setCreateModal(true)}
            className="press"
            style={{
              height: 36,
              padding: "0 16px",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 600,
              background: "var(--brass)",
              color: "#0E0C09",
              border: "none",
              cursor: "pointer",
            }}
          >
            Add table
          </button>
        )}
      </div>

      {/* Table list */}
      {tables.length === 0 ? (
        <EmptyState icon={QrCode} title="No tables yet" description="Add your first table to generate a QR code." />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {tables.map(table => (
            <TableRow
              key={table.id}
              table={table}
              canManage={canManage}
              onPrint={handlePrint}
              onRefreshQR={handleRefreshQR}
            />
          ))}
        </div>
      )}

      {/* Create modal */}
      {createModal && (
        <BottomSheet open onClose={() => setCreateModal(false)} title="Add table">
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Table identifier</p>
              <Input
                placeholder="e.g. T3, Table 12, Garden 2"
                value={identifier}
                onChange={e => setIdentifier(e.target.value)}
                onKeyDown={e => e.key === "Enter" && handleCreate()}
              />
            </div>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Capacity</p>
              <Input
                type="number"
                min={1}
                max={20}
                value={capacity}
                onChange={e => setCapacity(Number(e.target.value))}
              />
            </div>
            <button
              onClick={handleCreate}
              disabled={creating || !identifier.trim()}
              className="press"
              style={{
                height: 44,
                borderRadius: "var(--rad-md)",
                fontSize: 14,
                fontWeight: 600,
                background: "var(--brass)",
                color: "#0E0C09",
                border: "none",
                cursor: creating || !identifier.trim() ? "not-allowed" : "pointer",
                opacity: creating || !identifier.trim() ? 0.6 : 1,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                gap: 8,
              }}
            >
              {creating && <Loader2 size={14} className="animate-spin" />}
              Create table
            </button>
          </div>
        </BottomSheet>
      )}

      {/* Hidden print template — rendered when printTable is set */}
      {printTable && <PrintTemplate table={printTable} tenantSlug={slug} />}
    </div>
  )
}

// ─── Settings Tab ────────────────────────────────────────────────────────────

function SettingsTab() {
  const { branchId, token, role } = useStaffStore()
  const [timeoutMinutes, setTimeoutMinutes] = useState(120)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then((b) => setTimeoutMinutes(b.session_timeout_minutes))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [branchId, token])

  async function handleSave() {
    if (!branchId || !token) return
    setSaving(true)
    try {
      await staffApi.updateBranch(branchId, { session_timeout_minutes: timeoutMinutes }, token)
      toast.success("Settings saved")
    } catch {
      toast.error("Failed to save settings")
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
      <p className="eyebrow" style={{ marginBottom: 6 }}>Session timeout</p>
      <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
        Sessions are automatically closed after this many minutes of inactivity.
      </p>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <Input
          type="number"
          min={15}
          max={480}
          step={15}
          value={timeoutMinutes}
          onChange={(e) => setTimeoutMinutes(parseInt(e.target.value) || 120)}
          disabled={!canEdit}
          style={{ width: 90 }}
        />
        <span className="text-sm" style={{ color: "var(--ink-2)" }}>minutes</span>
        {canEdit && (
          <Button
            onClick={handleSave}
            disabled={saving}
            style={{ marginLeft: 8 }}
          >
            {saving ? <Loader2 className="size-4 animate-spin" /> : "Save"}
          </Button>
        )}
      </div>
    </HospitalityCard>
  )
}

// ─── Page ────────────────────────────────────────────────────────────────────

const TABS = [
  { id: "sessions", label: "Sessions" },
  { id: "menu",     label: "Menu"     },
  { id: "tables",   label: "Tables"   },
  { id: "staff",    label: "Staff"    },
  { id: "stats",    label: "Stats"    },
  { id: "plan",     label: "Plan"     },
  { id: "settings", label: "Settings" },
]

export default function AdminPage() {
  const [activeTab, setActiveTab] = useState("sessions")

  return (
    <div className="screen-enter bg-[var(--bg-base)] text-[var(--ink-1)] min-h-screen px-5 py-6">
      {/* Heading */}
      <p className="eyebrow">Operations</p>
      <h1 className="display-xl mt-1.5">House overview</h1>

      {/* Pill tab switcher */}
      <div className="hscroll mt-6 overflow-x-auto pb-0.5">
        <div className="inline-flex gap-0.5 p-[3px] bg-[var(--bg-elev-1)] rounded-full border border-[var(--line-1)]">
          {TABS.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id)}
              className={cn(
                "press px-[18px] py-2 rounded-full border-0 text-[13px] cursor-pointer whitespace-nowrap transition-[background,color,box-shadow] duration-[var(--dur-fast)]",
                activeTab === id
                  ? "font-semibold bg-[var(--bg-elev-3)] text-[var(--ink-1)] shadow-[var(--shadow-1)]"
                  : "font-normal bg-transparent text-[var(--ink-3)]"
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="screen-enter mt-5">
        {activeTab === "sessions" && <SessionsTab />}
        {activeTab === "menu"     && <MenuTab />}
        {activeTab === "tables"   && <TablesTab />}
        {activeTab === "staff"    && <StaffTab />}
        {activeTab === "stats"    && <StatsTab />}
        {activeTab === "plan"     && <PlanTab />}
        {activeTab === "settings" && <SettingsTab />}
      </div>
    </div>
  )
}
