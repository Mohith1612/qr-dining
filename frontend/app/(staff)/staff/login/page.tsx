"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { staffApi } from "@/lib/api/staff"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Building2, Loader2 } from "lucide-react"
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
      className="min-h-screen flex items-center justify-center px-6"
      style={{ backgroundColor: "var(--color-bg)" }}
    >
      <div className="w-full max-w-sm space-y-8">
        <div className="text-center space-y-3">
          <div
            className="size-14 rounded-2xl flex items-center justify-center mx-auto"
            style={{
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              boxShadow: "var(--shadow-card)",
            }}
          >
            <Building2 className="size-7" style={{ color: "var(--color-accent)" }} aria-hidden />
          </div>
          <div>
            <h1 className="text-xl font-semibold" style={{ color: "var(--color-text)" }}>
              Staff Portal
            </h1>
            <p className="text-sm mt-1" style={{ color: "var(--color-text-muted)" }}>
              Sign in with your branch ID and PIN
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label
              htmlFor="branch-id"
              className="text-xs font-medium uppercase tracking-wider"
              style={{ color: "var(--color-text-muted)" }}
            >
              Branch ID
            </label>
            <Input
              id="branch-id"
              type="number"
              inputMode="numeric"
              placeholder="1"
              value={branchId}
              onChange={(e) => setBranchId(e.target.value)}
              required
              min={1}
              className="h-12 text-base rounded-xl"
              style={{
                backgroundColor: "var(--color-surface)",
                borderColor: "var(--color-border)",
                color: "var(--color-text)",
              }}
            />
          </div>

          <div className="space-y-2">
            <label
              htmlFor="pin"
              className="text-xs font-medium uppercase tracking-wider"
              style={{ color: "var(--color-text-muted)" }}
            >
              PIN
            </label>
            <Input
              id="pin"
              type="password"
              inputMode="numeric"
              placeholder="••••••"
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              maxLength={6}
              required
              autoComplete="current-password"
              className="h-12 text-base rounded-xl tracking-widest"
              style={{
                backgroundColor: "var(--color-surface)",
                borderColor: "var(--color-border)",
                color: "var(--color-text)",
              }}
            />
          </div>

          <Button
            type="submit"
            disabled={loading || !branchId || !pin}
            className="w-full h-12 rounded-xl font-medium text-base mt-2"
            style={{
              backgroundColor: "var(--color-accent)",
              color: "var(--color-accent-fg)",
            }}
          >
            {loading ? (
              <span className="flex items-center gap-2">
                <Loader2 className="size-4 animate-spin" aria-hidden />
                Signing in…
              </span>
            ) : (
              "Sign in"
            )}
          </Button>
        </form>
      </div>
    </div>
  )
}
