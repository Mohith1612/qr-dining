"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useStaffStore } from "@/store/staff"
import { StaffBar } from "@/components/staff/StaffBar"

export default function StaffLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { token, _hydrated, clear } = useStaffStore()

  useEffect(() => {
    if (_hydrated && !token) {
      router.replace("/staff/login")
    }
  }, [token, _hydrated, router])

  if (!_hydrated || !token) return null

  function handleSignOut() {
    clear()
    router.replace("/staff/login")
  }

  return (
    <div className="min-h-screen flex flex-col" style={{ background: "var(--bg-base)", color: "var(--ink-1)" }}>
      <StaffBar onSignOut={handleSignOut} />
      <main className="flex-1">{children}</main>
    </div>
  )
}
