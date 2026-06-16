import { api } from "./client"
import type { Promo, ValidatePromoResponse } from "@/types/api"

interface CreatePromoRequest {
  code: string
  type: "flat_amount" | "percentage"
  value: number
  min_order_amount?: number
  max_uses?: number | null
  uses_per_phone?: number
  valid_from: string
  valid_until: string
  time_window_start?: string | null
  time_window_end?: string | null
  description?: string | null
}

export const promosApi = {
  validate: (sessionId: string, code: string, participantId?: number) =>
    api.post<ValidatePromoResponse>(
      `/sessions/${sessionId}/promos/validate`,
      { code },
      { participantId }
    ),

  list: (branchId: number, token: string) =>
    api.get<Promo[]>(`/branches/${branchId}/promos`, { staffToken: token }),

  create: (branchId: number, data: CreatePromoRequest, token: string) =>
    api.post<Promo>(`/branches/${branchId}/promos`, data, { staffToken: token }),

  deactivate: (branchId: number, promoId: number, token: string) =>
    api.delete<void>(`/branches/${branchId}/promos/${promoId}`, { staffToken: token }),
}
