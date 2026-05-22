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
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { formatCurrency, relativeTime } from "@/lib/format"
import { RefreshCw, Loader2, Users } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, StaffRole } from "@/types/api"

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
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          {sessions.length} active session{sessions.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={fetchSessions}
          className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg transition-opacity active:opacity-70"
          style={{ color: "var(--color-text-muted)" }}
          aria-label="Refresh sessions"
        >
          <RefreshCw className="size-3.5" aria-hidden />
          Refresh
        </button>
      </div>

      {sessions.length === 0 ? (
        <div
          className="rounded-2xl p-8 text-center"
          style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
        >
          <Users className="size-6 mx-auto mb-2" style={{ color: "var(--color-text-muted)" }} />
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            No active sessions
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {sessions.map((s) => (
            <div
              key={s.id}
              className="rounded-2xl p-4 flex items-center justify-between gap-3"
              style={{
                backgroundColor: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-lg)",
              }}
            >
              <div className="space-y-0.5 min-w-0">
                <p className="font-mono text-xs font-semibold truncate" style={{ color: "var(--color-text)" }}>
                  #{s.id.slice(-10)}
                </p>
                <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
                  Table {s.table_id} · {relativeTime(s.created_at)}
                </p>
              </div>
              <span
                className="text-xs font-medium px-2 py-0.5 rounded-full shrink-0"
                style={{
                  backgroundColor: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  color: "var(--color-text-muted)",
                }}
              >
                {s.status}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Menu Tab ────────────────────────────────────────────────────────────────

function MenuTab() {
  const { branchId, token } = useStaffStore()
  const [categories, setCategories] = useState<MenuCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [toggling, setToggling] = useState<number | null>(null)

  useEffect(() => {
    if (!branchId) return
    menuApi.getMenu(branchId).then(setCategories).catch(() => {
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

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {categories.map((cat) => (
        <section key={cat.id} className="space-y-2">
          <h3 className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-muted)" }}>
            {cat.name}
          </h3>
          <div className="space-y-1">
            {cat.items.map((item) => (
              <div
                key={item.id}
                className="flex items-center justify-between gap-3 px-4 py-3 rounded-xl"
                style={{
                  backgroundColor: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  opacity: item.is_available ? 1 : 0.6,
                }}
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate" style={{ color: "var(--color-text)" }}>
                    {item.name}
                  </p>
                  <p className="text-xs" style={{ color: "var(--color-text-muted)" }}>
                    {formatCurrency(item.price)}
                  </p>
                </div>
                <button
                  onClick={() => handleToggle(item.id, item.is_available)}
                  disabled={toggling === item.id}
                  className="shrink-0 text-xs font-medium px-3 py-1.5 rounded-full transition-opacity active:opacity-70 min-h-[36px]"
                  style={{
                    backgroundColor: item.is_available ? "var(--color-success)" : "var(--color-border)",
                    color: item.is_available ? "white" : "var(--color-text-muted)",
                  }}
                  aria-label={item.is_available ? "Mark unavailable" : "Mark available"}
                >
                  {toggling === item.id ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : item.is_available ? (
                    "Available"
                  ) : (
                    "Off"
                  )}
                </button>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}

// ─── Staff Tab ───────────────────────────────────────────────────────────────

const STAFF_ROLES: { value: StaffRole; label: string }[] = [
  { value: "waiter", label: "Waiter" },
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
      <div className="py-16 text-center" style={{ color: "var(--color-text-muted)" }}>
        <p className="text-sm">Only owners and managers can manage staff accounts.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div
        className="rounded-2xl p-5 space-y-4"
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
        }}
      >
        <h3 className="font-semibold text-sm" style={{ color: "var(--color-text)" }}>
          Add staff member
        </h3>

        <form onSubmit={handleCreate} className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-xs font-medium" style={{ color: "var(--color-text-muted)" }}>
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Staff name"
              required
              className="h-11 rounded-xl"
              style={{
                backgroundColor: "var(--color-bg)",
                borderColor: "var(--color-border)",
                color: "var(--color-text)",
              }}
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-medium" style={{ color: "var(--color-text-muted)" }}>
              Role
            </label>
            <div className="flex gap-2">
              {STAFF_ROLES.map(({ value, label }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setSelectedRole(value)}
                  className="flex-1 py-2 rounded-xl text-xs font-medium transition-colors"
                  style={{
                    backgroundColor:
                      selectedRole === value ? "var(--color-accent)" : "var(--color-bg)",
                    color: selectedRole === value ? "var(--color-accent-fg)" : "var(--color-text-muted)",
                    border: "1px solid var(--color-border)",
                    minHeight: "44px",
                  }}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-medium" style={{ color: "var(--color-text-muted)" }}>
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
              className="h-11 rounded-xl tracking-widest"
              style={{
                backgroundColor: "var(--color-bg)",
                borderColor: "var(--color-border)",
                color: "var(--color-text)",
              }}
            />
          </div>

          <Button
            type="submit"
            disabled={creating || !name || !pin}
            className="w-full h-11 rounded-xl font-medium text-sm"
            style={{ backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }}
          >
            {creating ? (
              <span className="flex items-center gap-2">
                <Loader2 className="size-4 animate-spin" />
                Creating…
              </span>
            ) : (
              "Create account"
            )}
          </Button>
        </form>
      </div>
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
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

  if (gated) {
    return (
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          padding: "32px 24px",
          textAlign: "center",
        }}
      >
        <p className="text-sm" style={{ color: "var(--color-text)", marginBottom: "8px" }}>
          Analytics is available on the Standard and Premium plans.
        </p>
        <Link
          href="/pricing"
          style={{
            fontSize: "13px",
            color: "var(--color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          View pricing →
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PeriodSelector value={period} onChange={setPeriod} />

      <div
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          padding: "20px",
        }}
      >
        <h3
          className="text-xs font-semibold uppercase tracking-wider"
          style={{ color: "var(--color-text-muted)", marginBottom: "16px" }}
        >
          Top items
        </h3>
        <TopItemsList items={topItems} />
      </div>

      <div
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          padding: "20px",
        }}
      >
        <h3
          className="text-xs font-semibold uppercase tracking-wider"
          style={{ color: "var(--color-text-muted)", marginBottom: "16px" }}
        >
          Busy hours
        </h3>
        <BusyHoursChart hours={busyHours} />
      </div>

      <div
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          padding: "20px",
        }}
      >
        <h3
          className="text-xs font-semibold uppercase tracking-wider"
          style={{ color: "var(--color-text-muted)", marginBottom: "16px" }}
        >
          Order volume
        </h3>
        <OrderVolumeChart days={orderVolume} />
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
  { label: "Branches", key: "max_branches" },
  { label: "Tables", key: "max_tables" },
  { label: "Analytics", key: "analytics" },
  { label: "Multi-branch", key: "multi_branch" },
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
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--color-text-muted)" }} />
      </div>
    )
  }

  if (!restaurantId) {
    return (
      <div className="py-16 text-center">
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          No tenant context. Set{" "}
          <code
            style={{
              fontSize: "11px",
              padding: "1px 5px",
              borderRadius: "4px",
              backgroundColor: "var(--color-border)",
            }}
          >
            NEXT_PUBLIC_TENANT_SLUG
          </code>{" "}
          in your local environment.
        </p>
      </div>
    )
  }

  if (!subscription || !features) {
    return (
      <div className="py-16 text-center">
        <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
          No subscription found.
        </p>
      </div>
    )
  }

  const tierBadgeStyle =
    subscription.plan_tier === "free"
      ? { backgroundColor: "var(--color-border)", color: "var(--color-text-muted)" }
      : { backgroundColor: "var(--color-accent)", color: "var(--color-accent-fg)" }

  const statusStyle =
    subscription.status === "active"
      ? { backgroundColor: "var(--color-success)", color: "white" }
      : subscription.status === "trial"
      ? { backgroundColor: "var(--color-border)", color: "var(--color-text)" }
      : { backgroundColor: "var(--color-border)", color: "var(--color-text-muted)" }

  return (
    <div className="space-y-4">
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          padding: "20px",
        }}
      >
        {/* Plan header */}
        <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px", flexWrap: "wrap" }}>
          <span
            style={{
              fontSize: "16px",
              fontWeight: 700,
              color: "var(--color-text)",
            }}
          >
            {subscription.plan_name}
          </span>
          <span
            style={{
              fontSize: "11px",
              fontWeight: 600,
              padding: "2px 8px",
              borderRadius: "100px",
              letterSpacing: "0.04em",
              textTransform: "uppercase",
              ...tierBadgeStyle,
            }}
          >
            {subscription.plan_tier}
          </span>
          <span
            style={{
              fontSize: "11px",
              fontWeight: 500,
              padding: "2px 8px",
              borderRadius: "100px",
              ...statusStyle,
            }}
          >
            {subscription.status}
          </span>
        </div>

        {subscription.status === "trial" && subscription.trial_ends_at && (
          <p style={{ fontSize: "13px", color: "var(--color-text-muted)", marginBottom: "16px" }}>
            Trial ends {formatDate(subscription.trial_ends_at)}
          </p>
        )}

        {/* Divider */}
        <div
          style={{
            height: "1px",
            backgroundColor: "var(--color-border)",
            margin: "16px 0",
          }}
        />

        {/* Feature list */}
        <div style={{ display: "flex", flexDirection: "column", gap: "10px", marginBottom: "20px" }}>
          {PLAN_FEATURES.map(({ label, key }) => {
            const val = features[key]
            return (
              <div
                key={key}
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  fontSize: "14px",
                }}
              >
                <span style={{ color: "var(--color-text-muted)" }}>{label}</span>
                <span
                  style={{
                    fontWeight: 500,
                    color:
                      (typeof val === "boolean" && !val) || val === 0
                        ? "var(--color-text-muted)"
                        : "var(--color-text)",
                  }}
                >
                  {formatFeatureVal(key, val)}
                </span>
              </div>
            )
          })}
        </div>

        {/* CTA */}
        <Link
          href="/pricing"
          style={{
            display: "block",
            textAlign: "center",
            padding: "9px 0",
            borderRadius: "var(--radius-base)",
            border: "1px solid var(--color-border)",
            fontSize: "13px",
            fontWeight: 500,
            color: "var(--color-text)",
            textDecoration: "none",
          }}
        >
          View pricing
        </Link>
      </div>
    </div>
  )
}

// ─── Page ────────────────────────────────────────────────────────────────────

export default function AdminPage() {
  return (
    <div
      className="px-4 py-6 space-y-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <h1 className="text-lg font-semibold">Admin dashboard</h1>

      <Tabs defaultValue="sessions">
        <div style={{ overflowX: "auto" }}>
          <TabsList
            className="inline-flex rounded-xl h-10"
            style={{
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              minWidth: "100%",
            }}
          >
            <TabsTrigger value="sessions" className="flex-1 text-xs rounded-lg">
              Sessions
            </TabsTrigger>
            <TabsTrigger value="menu" className="flex-1 text-xs rounded-lg">
              Menu
            </TabsTrigger>
            <TabsTrigger value="staff" className="flex-1 text-xs rounded-lg">
              Staff
            </TabsTrigger>
            <TabsTrigger value="stats" className="flex-1 text-xs rounded-lg">
              Stats
            </TabsTrigger>
            <TabsTrigger value="plan" className="flex-1 text-xs rounded-lg">
              Plan
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="sessions" className="mt-4">
          <SessionsTab />
        </TabsContent>
        <TabsContent value="menu" className="mt-4">
          <MenuTab />
        </TabsContent>
        <TabsContent value="staff" className="mt-4">
          <StaffTab />
        </TabsContent>
        <TabsContent value="stats" className="mt-4">
          <StatsTab />
        </TabsContent>
        <TabsContent value="plan" className="mt-4">
          <PlanTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
