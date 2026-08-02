"use client"

import { useState, useEffect } from "react"
import { KeyRound, Loader2, LogOut } from "lucide-react"
import { useStaffStore } from "@/store/staff"
import { useTenant } from "@/providers/TenantProvider"
import { useBrandingStore } from "@/store/branding"
import { staffApi } from "@/lib/api/staff"
import { ApiError } from "@/lib/api/client"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Badge, StatusDot, Button, Field, Input } from "@/components/ds"
import { toast } from "sonner"
import type { StaffRole } from "@/types/api"

const ROLE_LABEL: Record<StaffRole, string> = {
  kitchen: "Kitchen",
  waiter:  "Waiter",
  owner:   "Owner",
  manager: "Manager",
}

const ROLE_TONE: Record<StaffRole, "brand" | "info"> = {
  kitchen: "info",
  waiter:  "brand",
  owner:   "brand",
  manager: "brand",
}

interface StaffBarProps {
  onSignOut: () => void
}

export function StaffBar({ onSignOut }: StaffBarProps) {
  const { role, branchId, staffId, token } = useStaffStore()
  const { name: tenantName } = useTenant()
  const brandLogo = useBrandingStore((s) => s.logoUrl)
  const brandName = useBrandingStore((s) => s.name)
  const restaurantName = brandName ?? tenantName
  const [logoError, setLogoError] = useState(false)
  const [time, setTime] = useState(() => new Date())

  const [pinModal, setPinModal] = useState(false)
  const [currentPin, setCurrentPin] = useState("")
  const [newPin, setNewPin] = useState("")
  const [confirmPin, setConfirmPin] = useState("")
  const [savingPin, setSavingPin] = useState(false)

  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(id)
  }, [])

  const roleLabel = role ? ROLE_LABEL[role] : "Staff"
  const roleTone = role ? ROLE_TONE[role] : "brand"

  const pinValid = currentPin.length >= 4 && newPin.length >= 4 && newPin === confirmPin

  async function handleChangePin() {
    if (!staffId || !token || currentPin.length < 4 || newPin.length < 4) return
    if (newPin !== confirmPin) {
      toast.error("The new PIN and confirmation don't match.")
      return
    }
    setSavingPin(true)
    try {
      await staffApi.rotatePin(staffId, currentPin, newPin, token)
      toast.success("PIN updated")
      setPinModal(false)
      setCurrentPin("")
      setNewPin("")
      setConfirmPin("")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't change PIN.")
    } finally {
      setSavingPin(false)
    }
  }

  return (
    <header
      style={{
        height: 60, flexShrink: 0,
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "0 20px", borderBottom: "1px solid var(--line-1)",
        background: "color-mix(in oklch, var(--bg-elev-1) 80%, transparent)",
        backdropFilter: "blur(8px)", WebkitBackdropFilter: "blur(8px)",
        position: "sticky", top: 0, zIndex: 40,
      }}
    >
      {/* Left: brand mark (tenant logo when set) + name + role */}
      <div style={{ display: "inline-flex", alignItems: "center", gap: 14, minWidth: 0 }}>
        {(brandLogo && !logoError) ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={brandLogo}
            alt={restaurantName ?? "Restaurant"}
            onError={() => setLogoError(true)}
            style={{ width: 30, height: 30, borderRadius: "var(--rad-md)", objectFit: "contain", flexShrink: 0, background: "var(--bg-elev-2)", border: "1px solid var(--line-2)" }}
          />
        ) : (
          <span style={{
            width: 30, height: 30, borderRadius: "var(--rad-md)",
            background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
            display: "inline-flex", alignItems: "center", justifyContent: "center",
            color: "var(--accent)", flexShrink: 0,
          }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
              <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2"/>
              <path d="M7 2v20"/>
              <path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3Zm0 0v7"/>
            </svg>
          </span>
        )}
        <div style={{ minWidth: 0 }}>
          <div className="serif" style={{ fontSize: 17, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.01em", lineHeight: 1.05, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {restaurantName || "Restaurant"}
          </div>
          <div style={{ fontSize: 11, color: "var(--ink-3)", marginTop: 2 }}>
            {branchId ? `Branch ${branchId}` : "Staff portal"}
          </div>
        </div>
        <Badge tone={roleTone} style={{ marginLeft: 2 }}>{roleLabel}</Badge>
      </div>

      {/* Right: connection + time + actions */}
      <div style={{ display: "inline-flex", alignItems: "center", gap: 14, color: "var(--ink-3)", fontSize: 12 }}>
        <span className="hidden sm:inline-flex" style={{ alignItems: "center", gap: 6 }}>
          <StatusDot tone="ok" pulse />
          Connected
        </span>
        <span className="hidden sm:block" style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums", color: "var(--ink-2)" }}>
          {time.toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" })}
        </span>
        <Button variant="secondary" size="sm" aria-label="Change PIN" onClick={() => { setCurrentPin(""); setNewPin(""); setConfirmPin(""); setPinModal(true) }}>
          <KeyRound size={13} />
          <span className="hidden sm:inline">Change PIN</span>
        </Button>
        <Button variant="secondary" size="sm" aria-label="Sign out" onClick={onSignOut}>
          <LogOut size={13} />
          <span className="hidden sm:inline">Sign out</span>
        </Button>
      </div>

      {pinModal && (
        <BottomSheet open onClose={() => setPinModal(false)} title="Change my PIN">
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <Field label="Current PIN">
              <Input
                type="password"
                inputMode="numeric"
                value={currentPin}
                onChange={(e) => setCurrentPin(e.target.value.replace(/\D/g, ""))}
                placeholder="Current PIN"
                maxLength={6}
                style={{ letterSpacing: "0.2em" }}
              />
            </Field>
            <Field label="New PIN">
              <Input
                type="password"
                inputMode="numeric"
                value={newPin}
                onChange={(e) => setNewPin(e.target.value.replace(/\D/g, ""))}
                placeholder="4–6 digits"
                maxLength={6}
                style={{ letterSpacing: "0.2em" }}
              />
            </Field>
            <Field label="Confirm new PIN">
              <Input
                type="password"
                inputMode="numeric"
                value={confirmPin}
                onChange={(e) => setConfirmPin(e.target.value.replace(/\D/g, ""))}
                placeholder="Re-enter new PIN"
                maxLength={6}
                onKeyDown={(e) => e.key === "Enter" && handleChangePin()}
                style={{ letterSpacing: "0.2em" }}
              />
            </Field>
            {confirmPin.length > 0 && newPin !== confirmPin && (
              <p style={{ fontSize: 12, color: "var(--alert)", margin: "-6px 0 0" }}>PINs don&apos;t match.</p>
            )}
            <Button
              variant="brand"
              size="lg"
              onClick={handleChangePin}
              disabled={savingPin || !pinValid}
            >
              {savingPin && <Loader2 size={14} className="animate-spin" />}
              Update PIN
            </Button>
          </div>
        </BottomSheet>
      )}
    </header>
  )
}
