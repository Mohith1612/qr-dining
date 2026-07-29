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
import { RefreshCw, Loader2, Users, BarChart2, CreditCard, Printer, MoreVertical, RotateCcw, QrCode, Settings2, ChevronDown, ChevronRight, Plus, Trash2, Edit2, Tag, KeyRound, UserX } from "lucide-react"
import { toast } from "sonner"
import Link from "next/link"
import type { Session, MenuCategory, MenuItem, StaffRole, StaffRosterMember, Table, Promo } from "@/types/api"
import { promosApi } from "@/lib/api/promos"
import { tablesApi } from "@/lib/api/tables"
import { themeApi } from "@/lib/api/theme"
import { QRCard } from "@/components/admin/QRCard"
import { PrintTemplate } from "@/components/admin/PrintTemplate"
import { CollateralStudio } from "@/components/collateral/CollateralStudio"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { normalizeCollateralConfig, DEFAULT_COLLATERAL } from "@/types/collateral"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { MenuItemModal } from "@/components/admin/MenuItemModal"
import { StatsSkeleton, TablesSkeleton } from "@/components/shared/LoadingSkeleton"

export function PromosTab() {
  const { branchId, token } = useStaffStore()
  const [promos, setPromos] = useState<Promo[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)

  // Create form state
  const [code, setCode] = useState("")
  const [type, setType] = useState<"flat_amount" | "percentage">("percentage")
  const [value, setValue] = useState("")
  const [minOrder, setMinOrder] = useState("")
  const [maxUses, setMaxUses] = useState("")
  const [usesPerPhone, setUsesPerPhone] = useState("1")
  const [validFrom, setValidFrom] = useState("")
  const [validUntil, setValidUntil] = useState("")
  const [windowStart, setWindowStart] = useState("")
  const [windowEnd, setWindowEnd] = useState("")
  const [description, setDescription] = useState("")

  const fetchPromos = useCallback(async () => {
    if (!branchId || !token) return
    setLoading(true)
    try {
      const data = await promosApi.list(branchId, token)
      setPromos(data)
    } catch {
      toast.error("Couldn't load promos.")
    } finally {
      setLoading(false)
    }
  }, [branchId, token])

  useEffect(() => { fetchPromos() }, [fetchPromos])

  async function handleCreate() {
    if (!branchId || !token) return
    if (!code.trim() || !value || !validFrom || !validUntil) {
      toast.error("Code, value, and date range are required.")
      return
    }
    setCreating(true)
    try {
      await promosApi.create(branchId, {
        code: code.trim().toUpperCase(),
        type,
        value: parseFloat(value),
        min_order_amount: minOrder ? parseFloat(minOrder) : 0,
        max_uses: maxUses ? parseInt(maxUses) : null,
        uses_per_phone: usesPerPhone ? parseInt(usesPerPhone) : 0,
        // Date-only pickers: the promo runs from the start of the first day to
        // the end of the last day, in the operator's local time.
        valid_from: new Date(validFrom + "T00:00:00").toISOString(),
        valid_until: new Date(validUntil + "T23:59:59").toISOString(),
        time_window_start: windowStart || null,
        time_window_end: windowEnd || null,
        description: description || null,
      }, token)
      toast.success("Promo created.")
      setShowCreate(false)
      setCode(""); setValue(""); setMinOrder(""); setMaxUses(""); setUsesPerPhone("1")
      setValidFrom(""); setValidUntil(""); setWindowStart(""); setWindowEnd(""); setDescription("")
      fetchPromos()
    } catch {
      toast.error("Failed to create promo. Check the code isn't already in use.")
    } finally {
      setCreating(false)
    }
  }

  async function handleDeactivate(promoId: number) {
    if (!branchId || !token) return
    try {
      await promosApi.deactivate(branchId, promoId, token)
      toast.success("Promo deactivated.")
      fetchPromos()
    } catch {
      toast.error("Failed to deactivate promo.")
    }
  }

  function formatPromoValue(p: Promo) {
    return p.type === "percentage" ? `${p.value}% off` : `₹${p.value} off`
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="size-6 animate-spin" style={{ color: "var(--ink-3)" }} />
      </div>
    )
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <p style={{ fontSize: 13, color: "var(--ink-3)" }}>
          {promos.length} promo{promos.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={() => setShowCreate((v) => !v)}
          className="press"
          style={{
            display: "flex", alignItems: "center", gap: 6,
            fontSize: 12, color: showCreate ? "var(--accent)" : "var(--ink-3)",
            padding: "6px 10px", borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
          }}
        >
          <Plus size={12} aria-hidden />
          New promo
        </button>
      </div>

      {showCreate && (
        <HospitalityCard elev={1} style={{ padding: "16px 18px", display: "flex", flexDirection: "column", gap: 12 }}>
          <p className="eyebrow" style={{ marginBottom: 4 }}>Create promotion</p>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Code</label>
              <Input
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                placeholder="HAPPY20"
              />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Type</label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as "flat_amount" | "percentage")}
                style={{
                  width: "100%", height: 36, borderRadius: "var(--rad-md)",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-1)", fontSize: 13, padding: "0 8px",
                }}
              >
                <option value="percentage">Percentage %</option>
                <option value="flat_amount">Flat ₹</option>
              </select>
            </div>
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>
                Value ({type === "percentage" ? "%" : "₹"})
              </label>
              <Input type="number" value={value} onChange={(e) => setValue(e.target.value)} placeholder="20" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Min order (₹)</label>
              <Input type="number" value={minOrder} onChange={(e) => setMinOrder(e.target.value)} placeholder="0" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Max total uses</label>
              <Input type="number" value={maxUses} onChange={(e) => setMaxUses(e.target.value)} placeholder="∞" />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Uses per guest</label>
              <Input type="number" min={0} value={usesPerPhone} onChange={(e) => setUsesPerPhone(e.target.value)} placeholder="1" />
            </div>
          </div>
          <p style={{ fontSize: 11, color: "var(--ink-4)", marginTop: -4 }}>
            Uses per guest &gt; 0 requires guests to enter their phone number to redeem (the offer is tracked per number). Set 0 for unlimited / no phone.
          </p>

          <div style={{ display: "flex", gap: 8 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Valid from</label>
              <Input type="date" value={validFrom} onChange={(e) => setValidFrom(e.target.value)} />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Valid until</label>
              <Input type="date" value={validUntil} onChange={(e) => setValidUntil(e.target.value)} />
            </div>
          </div>
          <p style={{ fontSize: 11, color: "var(--ink-4)", marginTop: -4 }}>
            The promo is live from the start of the first day to the end of the last day.
          </p>

          <div style={{ borderTop: "1px solid var(--line-1)", paddingTop: 10 }}>
            <p style={{ fontSize: 12, fontWeight: 600, color: "var(--ink-2)", marginBottom: 2 }}>Active hours each day (optional)</p>
            <p style={{ fontSize: 11, color: "var(--ink-4)", marginBottom: 8 }}>
              e.g. 16:00–19:00 for a happy-hour offer. Leave blank to run all day.
            </p>
            <div style={{ display: "flex", gap: 8 }}>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>From</label>
                <Input type="time" value={windowStart} onChange={(e) => setWindowStart(e.target.value)} placeholder="16:00" />
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Until</label>
                <Input type="time" value={windowEnd} onChange={(e) => setWindowEnd(e.target.value)} placeholder="19:00" />
              </div>
            </div>
          </div>

          <div>
            <label style={{ fontSize: 11, color: "var(--ink-3)", display: "block", marginBottom: 4 }}>Description (shown to guests)</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Happy Hour — 20% off all beverages"
            />
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <Button onClick={handleCreate} disabled={creating} style={{ flex: 1 }}>
              {creating ? <Loader2 className="size-4 animate-spin" /> : "Create promo"}
            </Button>
            <Button
              onClick={() => setShowCreate(false)}
              style={{ flex: 1, background: "var(--bg-elev-2)", color: "var(--ink-2)", border: "1px solid var(--line-1)" }}
            >
              Cancel
            </Button>
          </div>
        </HospitalityCard>
      )}

      {promos.length === 0 ? (
        <EmptyState
          icon={Tag}
          title="No promos yet"
          description="Create a promo code to offer discounts to guests."
        />
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {promos.map((p) => (
            <HospitalityCard
              key={p.id}
              elev={1}
              style={{
                padding: "14px 16px",
                opacity: p.is_active ? 1 : 0.5,
              }}
            >
              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                    <p className="mono" style={{ fontSize: 13, fontWeight: 700, color: "var(--accent)", letterSpacing: "0.05em" }}>
                      {p.code}
                    </p>
                    <span style={{
                      fontSize: 11, fontWeight: 500, padding: "2px 8px",
                      borderRadius: "var(--rad-pill)",
                      background: p.is_active ? "var(--ok-soft)" : "var(--bg-elev-2)",
                      color: p.is_active ? "var(--ok)" : "var(--ink-3)",
                      border: `1px solid ${p.is_active ? "var(--ok)" : "var(--line-1)"}`,
                    }}>
                      {p.is_active ? "Active" : "Inactive"}
                    </span>
                  </div>
                  <p style={{ fontSize: 13, fontWeight: 500, color: "var(--ink-1)", marginBottom: 2 }}>
                    {formatPromoValue(p)}
                    {p.min_order_amount > 0 && ` · min ₹${p.min_order_amount}`}
                  </p>
                  {p.description && (
                    <p style={{ fontSize: 12, color: "var(--ink-3)", marginBottom: 4, lineHeight: 1.4 }}>{p.description}</p>
                  )}
                  <p style={{ fontSize: 11, color: "var(--ink-4)" }}>
                    {new Date(p.valid_from).toLocaleDateString()} – {new Date(p.valid_until).toLocaleDateString()}
                    {p.max_uses && ` · max ${p.max_uses} uses`}
                    {p.time_window_start && ` · ${p.time_window_start}–${p.time_window_end}`}
                  </p>
                </div>
                {p.is_active && (
                  <button
                    onClick={() => handleDeactivate(p.id)}
                    className="press"
                    style={{
                      flexShrink: 0, fontSize: 12, color: "var(--ink-3)",
                      padding: "5px 10px", borderRadius: "var(--rad-md)",
                      background: "var(--bg-elev-2)", border: "1px solid var(--line-1)",
                    }}
                  >
                    Deactivate
                  </button>
                )}
              </div>
            </HospitalityCard>
          ))}
        </div>
      )}
    </div>
  )
}

