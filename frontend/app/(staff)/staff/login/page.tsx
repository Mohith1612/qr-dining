"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { useTenant } from "@/providers/TenantProvider"
import { HospitalityCard } from "@/components/shared/HospitalityCard"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import type { StaffRole } from "@/types/api"

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
  const [branchId, setBranchId] = useState("")
  const [pin, setPin] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const id = parseInt(branchId)
    if (!id || !pin) return
    setLoading(true)
    try {
      const session = await staffApi.auth(id, pin)
      setAuth(session.token, session.staff_id, session.branch_id, session.role)
      router.replace(ROLE_REDIRECT[session.role])
    } catch {
      toast.error("Invalid branch ID or PIN. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
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
            {/* Branch ID */}
            <p className="eyebrow" style={{ marginBottom: 8 }}>Branch ID</p>
            <input
              type="number"
              inputMode="numeric"
              placeholder="1"
              value={branchId}
              onChange={(e) => setBranchId(e.target.value)}
              required
              min={1}
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
            <button
              type="submit"
              disabled={loading || !branchId || !pin}
              className="press"
              style={{
                display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
                width: "100%", height: 48,
                background: "var(--accent)", color: "var(--accent-ink)",
                border: "none", borderRadius: "var(--rad-md)",
                fontSize: 15, fontWeight: 600,
                cursor: loading || !branchId || !pin ? "not-allowed" : "pointer",
                opacity: loading || !branchId || !pin ? 0.55 : 1,
              }}
            >
              {loading ? (
                <>
                  <Loader2 className="animate-spin" style={{ width: 16, height: 16 }} />
                  Signing in…
                </>
              ) : "Sign in"}
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
