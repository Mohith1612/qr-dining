"use client"

import { useEffect, useState } from "react"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"

export function SettingsTab() {
  const { branchId, token, role } = useStaffStore()
  const [timeoutMinutes, setTimeoutMinutes] = useState(120)
  const [orderPrefix, setOrderPrefix] = useState("OR")
  const [prefixInput, setPrefixInput] = useState("OR")
  const [taxRateInput, setTaxRateInput] = useState("0")
  const [svcRateInput, setSvcRateInput] = useState("0")
  const [includeTaxInPrice, setIncludeTaxInPrice] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingPrefix, setSavingPrefix] = useState(false)
  const [savingBilling, setSavingBilling] = useState(false)
  const canEdit = role === "owner" || role === "manager"

  useEffect(() => {
    if (!branchId || !token) { setLoading(false); return }
    staffApi
      .getBranch(branchId, token)
      .then((b) => {
        setTimeoutMinutes(b.session_timeout_minutes)
        setOrderPrefix(b.order_prefix ?? "OR")
        setPrefixInput(b.order_prefix ?? "OR")
        setTaxRateInput(String(Math.round((b.tax_rate ?? 0) * 100)))
        setSvcRateInput(String(Math.round((b.service_charge_rate ?? 0) * 100)))
        setIncludeTaxInPrice(b.include_tax_in_price ?? false)
      })
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

  async function handleSavePrefix() {
    if (!branchId || !token) return
    const p = prefixInput.toUpperCase().trim()
    if (!/^[A-Z]{2,4}$/.test(p)) {
      toast.error("Prefix must be 2–4 uppercase letters")
      return
    }
    setSavingPrefix(true)
    try {
      await staffApi.updateBranch(branchId, { order_prefix: p }, token)
      setOrderPrefix(p)
      toast.success("Order prefix updated")
    } catch {
      toast.error("Failed to update prefix")
    } finally {
      setSavingPrefix(false)
    }
  }

  async function handleSaveBilling() {
    if (!branchId || !token) return
    const taxRate = parseFloat(taxRateInput) / 100
    const svcRate = parseFloat(svcRateInput) / 100
    if (isNaN(taxRate) || taxRate < 0 || taxRate > 0.5) {
      toast.error("Tax rate must be between 0% and 50%")
      return
    }
    if (isNaN(svcRate) || svcRate < 0 || svcRate > 0.5) {
      toast.error("Service charge must be between 0% and 50%")
      return
    }
    setSavingBilling(true)
    try {
      await staffApi.updateBranch(branchId, {
        tax_rate: taxRate,
        service_charge_rate: svcRate,
        include_tax_in_price: includeTaxInPrice,
      }, token)
      toast.success("Billing settings saved")
    } catch {
      toast.error("Failed to save billing settings")
    } finally {
      setSavingBilling(false)
    }
  }

  const taxRateNum = parseFloat(taxRateInput) || 0
  const billingPreview = taxRateNum > 0 && !includeTaxInPrice
    ? `An order of ₹100 will show as: ₹100 + ₹${taxRateNum} GST = ₹${100 + taxRateNum} total`
    : includeTaxInPrice
    ? "Menu prices already include tax — no additional tax is added."
    : "No tax is added to orders."

  if (loading) return null

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
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

      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Order numbers</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
          A short prefix added to every order number. Example: prefix <span className="mono">TM</span> → <span className="mono">#TM1001</span>.
          Counters reset daily. Changes take effect on the next order.
        </p>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <Input
            type="text"
            maxLength={4}
            value={prefixInput}
            onChange={(e) => setPrefixInput(e.target.value.toUpperCase())}
            disabled={!canEdit}
            style={{ width: 72, fontFamily: "var(--font-mono, monospace)", letterSpacing: "0.08em", textTransform: "uppercase" }}
            placeholder="OR"
          />
          {canEdit && (
            <Button
              onClick={handleSavePrefix}
              disabled={savingPrefix}
            >
              {savingPrefix ? <Loader2 className="size-4 animate-spin" /> : "Save"}
            </Button>
          )}
        </div>
        <p className="text-sm mono" style={{ marginTop: 10, color: "var(--ink-3)" }}>
          Preview: #{orderPrefix}1001, #{orderPrefix}1002, #{orderPrefix}1003&hellip;
        </p>
      </HospitalityCard>

      <HospitalityCard elev={1} style={{ padding: "20px 20px" }}>
        <p className="eyebrow" style={{ marginBottom: 6 }}>Billing</p>
        <p className="text-sm" style={{ color: "var(--ink-3)", marginBottom: 14 }}>
          Tax and service charge settings shown on the itemized bill.
        </p>

        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span className="text-sm" style={{ color: "var(--ink-2)", width: 140 }}>Tax rate (GST %)</span>
            <Input
              type="number"
              min={0}
              max={50}
              step={1}
              value={taxRateInput}
              onChange={(e) => setTaxRateInput(e.target.value)}
              disabled={!canEdit}
              style={{ width: 72 }}
            />
            <span className="text-sm" style={{ color: "var(--ink-3)" }}>%</span>
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span className="text-sm" style={{ color: "var(--ink-2)", width: 140 }}>Service charge %</span>
            <Input
              type="number"
              min={0}
              max={50}
              step={1}
              value={svcRateInput}
              onChange={(e) => setSvcRateInput(e.target.value)}
              disabled={!canEdit}
              style={{ width: 72 }}
            />
            <span className="text-sm" style={{ color: "var(--ink-3)" }}>%</span>
          </div>

          <label style={{ display: "flex", alignItems: "center", gap: 10, cursor: canEdit ? "pointer" : "default" }}>
            <input
              type="checkbox"
              checked={includeTaxInPrice}
              onChange={(e) => setIncludeTaxInPrice(e.target.checked)}
              disabled={!canEdit}
              style={{ width: 16, height: 16, accentColor: "var(--accent)" }}
            />
            <span className="text-sm" style={{ color: "var(--ink-2)" }}>
              Prices include tax (do not add tax on top)
            </span>
          </label>
        </div>

        <p className="text-sm" style={{ marginTop: 12, color: "var(--ink-3)", fontStyle: "italic", lineHeight: 1.5 }}>
          {billingPreview}
        </p>

        {canEdit && (
          <Button
            onClick={handleSaveBilling}
            disabled={savingBilling}
            style={{ marginTop: 14 }}
          >
            {savingBilling ? <Loader2 className="size-4 animate-spin" /> : "Save billing settings"}
          </Button>
        )}
      </HospitalityCard>
    </div>
  )
}

