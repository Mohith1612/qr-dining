"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { LogOut } from "lucide-react"

const ROLE_LABEL: Record<string, string> = {
  owner: "Owner",
  manager: "Manager",
  waiter: "Waiter",
  kitchen: "Kitchen",
}

export default function StaffLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { token, role, branchId, clear } = useStaffStore()

  useEffect(() => {
    if (!token) {
      router.replace("/staff/login")
    }
  }, [token, router])

  if (!token) return null

  function handleSignOut() {
    clear()
    router.replace("/staff/login")
  }

  return (
    <div
      className="min-h-screen flex flex-col"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <header
        className="sticky top-0 z-40 flex items-center justify-between px-4 h-14 border-b"
        style={{
          backgroundColor: "var(--color-surface)",
          borderColor: "var(--color-border)",
          boxShadow: "var(--shadow-card)",
        }}
      >
        <div className="flex items-center gap-3">
          <span
            className="text-xs font-semibold px-2 py-0.5 rounded-full"
            style={{
              backgroundColor: "var(--color-accent)",
              color: "var(--color-accent-fg)",
            }}
          >
            {role ? ROLE_LABEL[role] : "Staff"}
          </span>
          {branchId && (
            <span className="text-sm" style={{ color: "var(--color-text-muted)" }}>
              Branch {branchId}
            </span>
          )}
        </div>

        <button
          onClick={handleSignOut}
          className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg transition-opacity active:opacity-70"
          style={{ color: "var(--color-text-muted)" }}
          aria-label="Sign out"
        >
          <LogOut className="size-3.5" aria-hidden />
          Sign out
        </button>
      </header>

      <main className="flex-1">{children}</main>
    </div>
  )
}
