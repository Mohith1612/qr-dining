"use client"

import { useAssistanceStore } from "@/store/assistance"
import { assistanceApi } from "@/lib/api/assistance"
import { useSession } from "./useSession"
import type { AssistanceType } from "@/types/api"

export function useAssistance() {
  const requests = useAssistanceStore((s) => s.requests)
  const { session } = useSession()

  const active = requests.filter((r) => r.status !== "resolved")

  async function requestAssistance(type?: AssistanceType) {
    if (!session) return null
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) return null
    const req = await assistanceApi.request(
      session.id,
      session.table_id,
      guestToken,
      type
    )
    useAssistanceStore.getState().addRequest(req)
    return req
  }

  return { requests, active, requestAssistance }
}
