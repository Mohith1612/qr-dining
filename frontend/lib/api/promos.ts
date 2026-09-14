import { api } from "./client"
import type { Promo, RawPromo, ValidatePromoResponse } from "@/types/api"

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
  validate: (sessionId: string, code: string, orderTotal?: number, phoneE164?: string, guestToken?: string) =>
    api.post<ValidatePromoResponse>(
      `/sessions/${sessionId}/promos/validate`,
      { code, order_total: orderTotal ?? 0, phone_e164: phoneE164 || undefined },
      { guestToken }
    ),

  list: (branchId: number, token: string) =>
    api.get<Promo[]>(`/branches/${branchId}/promos`, { staffToken: token }),

  create: (branchId: number, data: CreatePromoRequest, token: string) =>
    api.post<RawPromo>(`/branches/${branchId}/promos`, data, { staffToken: token }),

  deactivate: (branchId: number, promoId: number, token: string) =>
    api.delete<void>(`/branches/${branchId}/promos/${promoId}`, { staffToken: token }),

  activate: (branchId: number, promoId: number, token: string) =>
    api.post<void>(`/branches/${branchId}/promos/${promoId}/activate`, {}, { staffToken: token }),

  update: (branchId: number, promoId: number, data: CreatePromoRequest, token: string) =>
    api.patch<RawPromo>(`/branches/${branchId}/promos/${promoId}`, data, { staffToken: token }),
}
