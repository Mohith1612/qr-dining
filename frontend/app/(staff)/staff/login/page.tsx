"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { useTenant } from "@/providers/TenantProvider"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { ApiError } from "@/lib/api/client"
import type { StaffRole } from "@/types/api"
import { identifyStaff } from "@/lib/product-analytics/identity"
import { track } from "@/lib/product-analytics/events"

const ROLE_REDIRECT: Record<StaffRole, string> = {
  kitchen: "/staff/kitchen",
  waiter: "/staff/waiter",
  owner: "/staff/admin",
  manager: "/staff/admin",
}

export default function StaffLoginPage() {
  const router = useRouter()
  const setAuth = useStaffStore((s) => s.setAuth)
  const { name: restaurantName } = useTenant()
  const [branchCode, setBranchCode] = useState("")
  const [staffCode, setStaffCode] = useState("")
  const [pin, setPin] = useState("")
  const [loading, setLoading] = useState(false)
  const [lockedFor, setLockedFor] = useState(0) // seconds remaining on a lockout

  // Tick the lockout countdown down to zero.
  useEffect(() => {
    if (lockedFor <= 0) return
    const id = setInterval(() => setLockedFor((s) => Math.max(0, s - 1)), 1000)
    return () => clearInterval(id)
  }, [lockedFor])

  const locked = lockedFor > 0
  const lockLabel = `${Math.floor(lockedFor / 60)}:${String(lockedFor % 60).padStart(2, "0")}`

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!branchCode.trim() || !staffCode.trim() || !pin || locked) return
    setLoading(true)
    try {
      const session = await staffApi.auth(branchCode.trim(), staffCode.trim(), pin)
      setAuth(session.token, session.staff_id, session.branch_id, session.role)
      identifyStaff(session.staff_id, session.role, session.branch_id)
      track("staff_login_succeeded", { role: session.role, branch_id: session.branch_id })
      router.replace(ROLE_REDIRECT[session.role])
    } catch (err) {
      track("staff_login_failed", { error_code: err instanceof ApiError ? err.code : "UNKNOWN" })
      if (err instanceof ApiError && err.status === 423) {
        setLockedFor(err.retryAfter && err.retryAfter > 0 ? err.retryAfter : 120)
        toast.error("Too many attempts. The correct PIN will work again once the timer ends.")
      } else {
        toast.error("Invalid credentials. Please check your branch code, staff code, and PIN.")
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      data-surface="ops"
      className="atmos min-h-screen flex items-center justify-center px-6 screen-enter"
      style={{ background: "var(--bg-base)", color: "var(--ink-1)", backgroundImage: "var(--glow-warm)" }}
    >
      <div style={{ width: "100%", maxWidth: 420 }}>
        {/* Brand mark */}
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 16, marginBottom: 32 }}>
          <div style={{
            width: 64, height: 64,
            background: "linear-gradient(160deg, var(--bg-elev-2), var(--bg-elev-1))",
            border: "2px solid var(--line-2)",
            borderRadius: "var(--rad-xl)",
            boxShadow: "var(--shadow-3)",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}>
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2" />
              <path d="M7 2v20" />
              <path d="M21 15V2v0a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3zm0 0v7" />
            </svg>
          </div>
          <div style={{ textAlign: "center", display: "flex", flexDirection: "column", gap: 4 }}>
            <p className="eyebrow">{restaurantName ?? "Staff"} · Staff</p>
            <h1 className="serif" style={{ fontSize: 30, fontWeight: 500, color: "var(--ink-1)", margin: 0 }}>
              Welcome back
            </h1>
            <p style={{ fontSize: 13, color: "var(--ink-3)", margin: 0 }}>
              Sign in with your branch ID and PIN
            </p>
          </div>
        </div>

        {/* Form card */}
        <HospitalityCard elev={3} style={{ padding: "28px 24px" }}>
          <form onSubmit={handleSubmit}>
            {/* Branch Code */}
            <p className="eyebrow" style={{ marginBottom: 8 }}>Branch Code</p>
            <input
              data-ph-mask
              type="text"
              autoComplete="username"
              placeholder="main-restaurant"
              value={branchCode}
              onChange={(e) => setBranchCode(e.target.value)}
              required
              style={{
                display: "block", width: "100%", fontSize: 18, boxSizing: "border-box",
                background: "transparent", outline: "none", border: "none",
                borderBottom: "1px solid var(--line-2)", paddingBottom: 10,
                color: "var(--ink-1)", fontFamily: "inherit",
              }}
            />

            <div style={{ height: 20 }} />

            {/* Staff Code */}
            <p className="eyebrow" style={{ marginBottom: 8 }}>Staff Code</p>
            <input
              data-ph-mask
              type="text"
              autoComplete="username"
              placeholder="your-staff-code"
              value={staffCode}
              onChange={(e) => setStaffCode(e.target.value)}
              required
              style={{
                display: "block", width: "100%", fontSize: 18, boxSizing: "border-box",
                background: "transparent", outline: "none", border: "none",
                borderBottom: "1px solid var(--line-2)", paddingBottom: 10,
                color: "var(--ink-1)", fontFamily: "inherit",
              }}
            />

            <div style={{ height: 20 }} />

            {/* PIN display + hidden input */}
            <p className="eyebrow" style={{ marginBottom: 10 }}>PIN</p>
            <div style={{ position: "relative" }}>
              <div style={{ display: "flex", gap: 8 }} aria-hidden>
                {Array.from({ length: 6 }).map((_, i) => (
                  <div
                    key={i}
                    style={{
                      width: 48, height: 48, flexShrink: 0,
                      background: "var(--bg-elev-1)",
                      border: "1px solid var(--line-2)",
                      borderRadius: "var(--rad-md)",
                      display: "flex", alignItems: "center", justifyContent: "center",
                      fontSize: 20, color: "var(--ink-1)",
                    }}
                  >
                    {pin[i] ? "●" : ""}
                  </div>
                ))}
              </div>
              <input
                data-ph-mask
                type="password"
                inputMode="numeric"
                autoComplete="current-password"
                value={pin}
                onChange={(e) => setPin(e.target.value.replace(/\D/g, "").slice(0, 6))}
                aria-label="PIN"
                style={{
                  position: "absolute", inset: 0, opacity: 0,
                  cursor: "text", width: "100%", height: "100%",
                }}
              />
            </div>

            <div style={{ height: 24 }} />

            {/* Submit */}
            {locked && (
              <p style={{ textAlign: "center", fontSize: 13, color: "var(--alert)", margin: "0 0 12px" }} role="alert" aria-live="polite">
                Too many attempts. Try again in <span style={{ fontVariantNumeric: "tabular-nums", fontWeight: 700 }}>{lockLabel}</span>.
              </p>
            )}
            <button
              type="submit"
              disabled={loading || locked || !branchCode || !staffCode || !pin}
              className="press"
              style={{
                display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
                width: "100%", height: 48,
                background: "var(--accent)", color: "var(--accent-ink)",
                border: "none", borderRadius: "var(--rad-md)",
                fontSize: 15, fontWeight: 600,
                cursor: loading || locked || !branchCode || !staffCode || !pin ? "not-allowed" : "pointer",
                opacity: loading || locked || !branchCode || !staffCode || !pin ? 0.55 : 1,
              }}
            >
              {loading ? (
                <>
                  <Loader2 className="animate-spin" style={{ width: 16, height: 16 }} />
                  Signing in…
                </>
              ) : locked ? `Locked · ${lockLabel}` : "Sign in"}
            </button>
          </form>
        </HospitalityCard>

        <p style={{ textAlign: "center", marginTop: 16, fontSize: 12, color: "var(--ink-4)" }}>
          Contact your manager if you&apos;ve forgotten your PIN
        </p>
      </div>
    </div>
  )
}
