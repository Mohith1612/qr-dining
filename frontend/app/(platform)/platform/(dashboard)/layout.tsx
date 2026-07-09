"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { usePlatformStore } from "@/store/platform"
import { platformApi } from "@/lib/api/platform"
import { PlatformShell } from "@/components/platform/PlatformShell"

export default function PlatformDashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { token, _hydrated, clear } = usePlatformStore()

  useEffect(() => {
    if (_hydrated && !token) {
      router.replace("/platform/login")
    }
  }, [token, _hydrated, router])

  if (!_hydrated || !token) return null

  async function handleSignOut() {
    if (token) {
      try {
        await platformApi.logout(token)
      } catch {}
    }
    clear()
    router.replace("/platform/login")
  }

  return <PlatformShell onSignOut={handleSignOut}>{children}</PlatformShell>
}
