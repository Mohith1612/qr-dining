"use client"

import { useState, useEffect } from "react"
import { KeyRound, Loader2 } from "lucide-react"
import { useStaffStore } from "@/store/staff"
import { useTenant } from "@/providers/TenantProvider"
import { staffApi } from "@/lib/api/staff"
import { ApiError } from "@/lib/api/client"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Input } from "@/components/ui/input"
import { toast } from "sonner"
import type { StaffRole } from "@/types/api"

const ROLE_LABEL: Record<StaffRole, string> = {
  kitchen: "Kitchen",
  waiter:  "Waiter",
  owner:   "Owner",
  manager: "Manager",
}

const ROLE_COLORS: Record<string, { bg: string; fg: string }> = {
  Kitchen: { bg: "var(--info-soft)",   fg: "var(--info)"   },
  Waiter:  { bg: "var(--accent-soft)", fg: "var(--accent)" },
  Owner:   { bg: "var(--accent-soft)", fg: "var(--accent)" },
  Manager: { bg: "var(--accent-soft)", fg: "var(--accent)" },
}

interface StaffBarProps {
  onSignOut: () => void
}

export function StaffBar({ onSignOut }: StaffBarProps) {
  const { role, branchId, staffId, token } = useStaffStore()
  const { name: restaurantName } = useTenant()
  const [time, setTime] = useState(() => new Date())

  const [pinModal, setPinModal] = useState(false)
  const [currentPin, setCurrentPin] = useState("")
  const [newPin, setNewPin] = useState("")
  const [savingPin, setSavingPin] = useState(false)

  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(id)
  }, [])

  const roleLabel = role ? ROLE_LABEL[role] : "Staff"
  const roleColor = ROLE_COLORS[roleLabel] ?? ROLE_COLORS.Owner

  async function handleChangePin() {
    if (!staffId || !token || currentPin.length < 4 || newPin.length < 4) return
    setSavingPin(true)
    try {
      await staffApi.rotatePin(staffId, currentPin, newPin, token)
      toast.success("PIN updated")
      setPinModal(false)
      setCurrentPin("")
      setNewPin("")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't change PIN.")
    } finally {
      setSavingPin(false)
    }
  }

  return (
    <div style={{
      height: 64, flexShrink: 0,
      display: "flex", alignItems: "center", justifyContent: "space-between",
      padding: "0 24px", borderBottom: "1px solid var(--line-1)",
      background: "linear-gradient(180deg, var(--bg-elev-1), var(--bg-base))",
      position: "sticky", top: 0, zIndex: 40,
    }}>
      {/* Left: brand mark + name + role */}
      <div style={{ display: "inline-flex", alignItems: "center", gap: 16 }}>
        <span style={{
          width: 32, height: 32, borderRadius: 10,
          background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
          display: "inline-flex", alignItems: "center", justifyContent: "center",
          color: "var(--accent)", flexShrink: 0,
        }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
            <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2"/>
            <path d="M7 2v20"/>
            <path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3Zm0 0v7"/>
          </svg>
        </span>
        <div>
          <div className="serif" style={{ fontSize: 16, fontWeight: 500, color: "var(--ink-1)", letterSpacing: "-0.005em", lineHeight: 1 }}>
            {restaurantName || "Restaurant"}
          </div>
          <div style={{ fontSize: 11, color: "var(--ink-3)", marginTop: 3 }}>
            {branchId ? `Branch ${branchId}` : "Staff portal"}
          </div>
        </div>
        <span style={{
          marginLeft: 4, padding: "4px 10px", borderRadius: 999,
          background: roleColor.bg, color: roleColor.fg,
          fontSize: 11, letterSpacing: "0.10em", textTransform: "uppercase", fontWeight: 600,
        }}>
          {roleLabel}
        </span>
      </div>

      {/* Right: connection + time + sign out */}
      <div style={{ display: "inline-flex", alignItems: "center", gap: 18, color: "var(--ink-3)", fontSize: 12 }}>
        <span className="hidden sm:inline-flex" style={{ alignItems: "center", gap: 6 }}>
          <span className="live-dot" />
          Connected
        </span>
        <span className="hidden sm:block" style={{ fontVariantNumeric: "tabular-nums", color: "var(--ink-2)" }}>
          {time.toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" })}
        </span>
        <button
          onClick={() => { setCurrentPin(""); setNewPin(""); setPinModal(true) }}
          className="press"
          style={{
            display: "inline-flex", alignItems: "center", gap: 6,
            padding: "6px 12px", borderRadius: 8, border: "1px solid var(--line-2)",
            background: "transparent", color: "var(--ink-2)", fontSize: 12,
          }}
        >
          <KeyRound size={13} />
          <span className="hidden sm:inline">Change PIN</span>
        </button>
        <button
          onClick={onSignOut}
          className="press"
          style={{
            display: "inline-flex", alignItems: "center", gap: 6,
            padding: "6px 12px", borderRadius: 8, border: "1px solid var(--line-2)",
            background: "transparent", color: "var(--ink-2)", fontSize: 12,
          }}
        >
          Sign out
        </button>
      </div>

      {pinModal && (
        <BottomSheet open onClose={() => setPinModal(false)} title="Change my PIN">
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>Current PIN</p>
              <Input
                type="password"
                inputMode="numeric"
                value={currentPin}
                onChange={(e) => setCurrentPin(e.target.value.replace(/\D/g, ""))}
                placeholder="Current PIN"
                maxLength={6}
                style={{ letterSpacing: "0.2em" }}
              />
            </div>
            <div>
              <p className="eyebrow" style={{ marginBottom: 6 }}>New PIN</p>
              <Input
                type="password"
                inputMode="numeric"
                value={newPin}
                onChange={(e) => setNewPin(e.target.value.replace(/\D/g, ""))}
                placeholder="4–6 digits"
                maxLength={6}
                onKeyDown={(e) => e.key === "Enter" && handleChangePin()}
                style={{ letterSpacing: "0.2em" }}
              />
            </div>
            <button
              onClick={handleChangePin}
              disabled={savingPin || currentPin.length < 4 || newPin.length < 4}
              className="press"
              style={{
                height: 44, borderRadius: "var(--rad-md)", fontSize: 14, fontWeight: 600,
                background: "var(--brass)", color: "#0E0C09", border: "none",
                cursor: savingPin || currentPin.length < 4 || newPin.length < 4 ? "not-allowed" : "pointer",
                opacity: savingPin || currentPin.length < 4 || newPin.length < 4 ? 0.6 : 1,
                display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
              }}
            >
              {savingPin && <Loader2 size={14} className="animate-spin" />}
              Update PIN
            </button>
          </div>
        </BottomSheet>
      )}
    </div>
  )
}
