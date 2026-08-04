"use client"

import { useCallback, useEffect, useState } from "react"
import { useStaffStore } from "@/store/staff"
import {
  loyaltyApi,
  type LoyaltyProgram,
  type LoyaltyCustomerLookup,
  type LoyaltyTransaction,
  type LoyaltyAnalytics,
} from "@/lib/api/loyalty"
import { ApiError } from "@/lib/api/client"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { SectionHeader } from "@/components/shared/SectionHeader"
import { EmptyState } from "@/components/shared/EmptyState"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Gift, Lock, Search, Loader2 } from "lucide-react"
import { toast } from "sonner"
import { track } from "@/lib/product-analytics/events"

// Manager-facing loyalty: program rule (X points per ₹Y), customer balance
// lookup, ledger-only redeem/adjust, and a participation summary. Redemption
// records a points deduction only — any discount is applied off-system.
export function LoyaltyTab() {
  const { branchId, token, role } = useStaffStore()
  const canManage = role === "owner" || role === "manager"

  const [gated, setGated] = useState(false)
  const [loading, setLoading] = useState(true)
  const [program, setProgram] = useState<LoyaltyProgram | null>(null)
  const [analytics, setAnalytics] = useState<LoyaltyAnalytics | null>(null)

  // Program form state
  const [isActive, setIsActive] = useState(false)
  const [ratePoints, setRatePoints] = useState("1")
  const [rateAmount, setRateAmount] = useState("100.00")
  const [saving, setSaving] = useState(false)

  // Lookup state
  const [phone, setPhone] = useState("")
  const [looking, setLooking] = useState(false)
  const [lookup, setLookup] = useState<LoyaltyCustomerLookup | null>(null)
  const [transactions, setTransactions] = useState<LoyaltyTransaction[]>([])

  // Redeem / adjust form state
  const [redeemPoints, setRedeemPoints] = useState("")
  const [adjustDelta, setAdjustDelta] = useState("")
  const [adjustReason, setAdjustReason] = useState("")
  const [acting, setActing] = useState(false)

  const fetchBase = useCallback(async () => {
    if (!branchId || !token || !canManage) return
    setLoading(true)
    setGated(false)
    try {
      const [p, a] = await Promise.all([
        loyaltyApi.getProgram(branchId, token),
        loyaltyApi.getAnalytics(branchId, "weekly", token),
      ])
      setProgram(p)
      setIsActive(p.is_active)
      setRatePoints(String(p.earn_rate_points))
      setRateAmount(p.earn_rate_amount)
      setAnalytics(a)
    } catch (err) {
      if (err instanceof ApiError && err.code === "LOYALTY_DISABLED") {
        track("upsell_gate_viewed", { feature: "loyalty" })
        setGated(true)
      } else {
        toast.error("Couldn't load loyalty settings.")
      }
    } finally {
      setLoading(false)
    }
  }, [branchId, token, canManage])

  useEffect(() => {
    fetchBase()
  }, [fetchBase])

  const saveProgram = async () => {
    if (!branchId || !token) return
    const points = parseInt(ratePoints, 10)
    if (!points || points <= 0) {
      toast.error("Earn rate points must be a positive number.")
      return
    }
    setSaving(true)
    try {
      const p = await loyaltyApi.putProgram(
        branchId,
        { is_active: isActive, earn_rate_points: points, earn_rate_amount: rateAmount },
        token
      )
      setProgram(p)
      toast.success("Loyalty program saved.")
    } catch (err) {
      if (err instanceof ApiError && err.code === "VALIDATION_ERROR") {
        toast.error("Check the earn rate values.")
      } else {
        toast.error("Couldn't save the program.")
      }
    } finally {
      setSaving(false)
    }
  }

  const refreshAccount = async (lk: LoyaltyCustomerLookup) => {
    if (!branchId || !token) return
    setLookup(lk)
    if (lk.account) {
      try {
        const t = await loyaltyApi.listTransactions(branchId, lk.account.account_id, token)
        setTransactions(t.transactions)
      } catch {
        setTransactions([])
      }
    } else {
      setTransactions([])
    }
  }

  const doLookup = async () => {
    if (!branchId || !token || !phone.trim()) return
    setLooking(true)
    try {
      const lk = await loyaltyApi.lookupCustomer(branchId, phone.trim(), token)
      await refreshAccount(lk)
    } catch (err) {
      setLookup(null)
      setTransactions([])
      if (err instanceof ApiError && err.code === "CUSTOMER_NOT_FOUND") {
        toast.error("No customer with that phone number.")
      } else if (err instanceof ApiError && err.code === "INVALID_PHONE") {
        toast.error("That phone number doesn't look valid.")
      } else {
        toast.error("Lookup failed.")
      }
    } finally {
      setLooking(false)
    }
  }

  const doRedeem = async () => {
    if (!branchId || !token || !lookup?.account) return
    const points = parseInt(redeemPoints, 10)
    if (!points || points <= 0) {
      toast.error("Enter the points to redeem.")
      return
    }
    setActing(true)
    try {
      const account = await loyaltyApi.redeem(branchId, lookup.account.account_id, points, "counter redemption", token)
      toast.success(`Redeemed ${points} points. Apply the discount at the counter.`)
      setRedeemPoints("")
      await refreshAccount({ ...lookup, account })
    } catch (err) {
      if (err instanceof ApiError && err.code === "LOYALTY_INSUFFICIENT_POINTS") {
        toast.error("Not enough points.")
      } else if (err instanceof ApiError && err.code === "LOYALTY_DISABLED") {
        track("upsell_gate_viewed", { feature: "loyalty" })
        toast.error("Redemption is not enabled for this organization.")
      } else {
        toast.error("Redeem failed.")
      }
    } finally {
      setActing(false)
    }
  }

  const doAdjust = async () => {
    if (!branchId || !token || !lookup?.account) return
    const delta = parseInt(adjustDelta, 10)
    if (!delta) {
      toast.error("Enter a non-zero adjustment.")
      return
    }
    if (!adjustReason.trim()) {
      toast.error("A reason is required for adjustments.")
      return
    }
    setActing(true)
    try {
      const account = await loyaltyApi.adjust(branchId, lookup.account.account_id, delta, adjustReason.trim(), token)
      toast.success("Points adjusted.")
      setAdjustDelta("")
      setAdjustReason("")
      await refreshAccount({ ...lookup, account })
    } catch (err) {
      if (err instanceof ApiError && err.code === "LOYALTY_INSUFFICIENT_POINTS") {
        toast.error("Balance can't go below zero.")
      } else if (err instanceof ApiError && err.code === "LOYALTY_DISABLED") {
        track("upsell_gate_viewed", { feature: "loyalty" })
        toast.error("Manual adjustments are not enabled for this organization.")
      } else {
        toast.error("Adjustment failed.")
      }
    } finally {
      setActing(false)
    }
  }

  if (!canManage) {
    return (
      <EmptyState
        icon={Lock}
        title="Owner or manager only"
        description="Loyalty settings are visible to owners and managers."
      />
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16 opacity-60">
        <Loader2 size={20} className="animate-spin" />
      </div>
    )
  }

  if (gated) {
    return (
      <EmptyState
        icon={Gift}
        title="Loyalty not enabled"
        description="Customer loyalty is not enabled for this organization. Ask your platform operator to enable it."
      />
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Earning rule</SectionHeader>
        <div className="flex flex-col gap-3 max-w-md">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={isActive}
              onChange={(e) => setIsActive(e.target.checked)}
            />
            Loyalty program active
          </label>
          <div className="flex items-center gap-2 text-sm">
            <span className="opacity-60 whitespace-nowrap">Earn</span>
            <Input
              value={ratePoints}
              onChange={(e) => setRatePoints(e.target.value)}
              inputMode="numeric"
              className="w-20"
              aria-label="Points earned"
            />
            <span className="opacity-60 whitespace-nowrap">point(s) per ₹</span>
            <Input
              value={rateAmount}
              onChange={(e) => setRateAmount(e.target.value)}
              inputMode="decimal"
              className="w-28"
              aria-label="Spend per points"
            />
            <span className="opacity-60 whitespace-nowrap">spent</span>
          </div>
          <p className="text-xs opacity-50">
            Points accrue automatically when a payment completes for a session with a linked
            customer. Redemption is recorded here; apply the discount at the counter.
          </p>
          <Button onClick={saveProgram} disabled={saving} className="self-start">
            {saving ? "Saving…" : program?.configured ? "Save changes" : "Create program"}
          </Button>
        </div>
      </HospitalityCard>

      {analytics && (
        <HospitalityCard elev={1} className="p-5">
          <SectionHeader className="mb-4">This week</SectionHeader>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3 text-sm">
            <Stat label="Points issued" value={analytics.points_issued} />
            <Stat label="Points redeemed" value={analytics.points_redeemed} />
            <Stat label="Active members" value={analytics.active_customers} />
            <Stat label="Total members" value={analytics.total_accounts} />
            <Stat label="Loyalty sessions" value={analytics.loyalty_sessions} />
            <Stat label="All sessions" value={analytics.total_sessions} />
          </div>
          {analytics.top_customers.length > 0 && (
            <div className="mt-4">
              <p className="text-xs opacity-60 mb-2">Top customers</p>
              <div className="flex flex-col gap-1 text-sm">
                {analytics.top_customers.map((c) => (
                  <div key={c.account_id} className="flex justify-between border-t border-white/5 py-1.5">
                    <span>
                      {c.display_name || c.phone_e164}
                      <span className="ml-2 text-xs opacity-50">{c.visit_count} visits</span>
                    </span>
                    <span className="opacity-80">{c.points_balance} pts</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </HospitalityCard>
      )}

      <HospitalityCard elev={1} className="p-5">
        <SectionHeader className="mb-4">Member lookup</SectionHeader>
        <div className="flex gap-2 max-w-md">
          <Input
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="+91 phone number"
            inputMode="tel"
            onKeyDown={(e) => e.key === "Enter" && doLookup()}
          />
          <Button onClick={doLookup} disabled={looking}>
            {looking ? <Loader2 size={16} className="animate-spin" /> : <Search size={16} />}
          </Button>
        </div>

        {lookup && (
          <div className="mt-4 flex flex-col gap-4">
            <div>
              <p className="font-medium">
                {lookup.display_name || "Unnamed customer"}
                <span className="ml-2 text-xs opacity-50">{lookup.phone_e164}</span>
              </p>
              {lookup.account ? (
                <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
                  <Stat label="Balance" value={lookup.account.points_balance} />
                  <Stat label="Lifetime earned" value={lookup.account.lifetime_points_earned} />
                  <Stat label="Visits" value={lookup.account.visit_count} />
                  <Stat label="Lifetime spend" value={`₹${lookup.account.lifetime_spend}`} />
                </div>
              ) : (
                <p className="text-sm opacity-60 mt-2">No loyalty activity yet.</p>
              )}
            </div>

            {lookup.account && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="rounded-lg border border-white/10 p-4 flex flex-col gap-2">
                  <p className="text-sm font-medium">Redeem points</p>
                  <div className="flex gap-2">
                    <Input
                      value={redeemPoints}
                      onChange={(e) => setRedeemPoints(e.target.value)}
                      placeholder="Points"
                      inputMode="numeric"
                    />
                    <Button onClick={doRedeem} disabled={acting}>
                      Redeem
                    </Button>
                  </div>
                  <p className="text-xs opacity-50">
                    Records the deduction; apply the discount at the counter.
                  </p>
                </div>
                <div className="rounded-lg border border-white/10 p-4 flex flex-col gap-2">
                  <p className="text-sm font-medium">Manual adjustment</p>
                  <div className="flex gap-2">
                    <Input
                      value={adjustDelta}
                      onChange={(e) => setAdjustDelta(e.target.value)}
                      placeholder="+/- points"
                      inputMode="numeric"
                    />
                    <Input
                      value={adjustReason}
                      onChange={(e) => setAdjustReason(e.target.value)}
                      placeholder="Reason (required)"
                    />
                    <Button onClick={doAdjust} disabled={acting} variant="outline">
                      Apply
                    </Button>
                  </div>
                </div>
              </div>
            )}

            {transactions.length > 0 && (
              <div>
                <p className="text-xs opacity-60 mb-2">Recent activity</p>
                <div className="flex flex-col text-sm">
                  {transactions.map((t) => (
                    <div key={t.id} className="flex justify-between border-t border-white/5 py-1.5">
                      <span>
                        <span className="capitalize">{t.type}</span>
                        {t.reason && <span className="ml-2 text-xs opacity-50">{t.reason}</span>}
                      </span>
                      <span className={t.points >= 0 ? "text-emerald-400" : "text-rose-400"}>
                        {t.points >= 0 ? `+${t.points}` : t.points} pts
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </HospitalityCard>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <p className="text-xs opacity-60">{label}</p>
      <p className="text-lg font-semibold">{value}</p>
    </div>
  )
}
