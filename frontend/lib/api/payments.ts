import { api } from "./client"
import type { Payment, PendingPayment, PaymentMethod, BillData } from "@/types/api"

export const paymentsApi = {
  initiate: (
    sessionId: string,
    amount: number,
    method: PaymentMethod,
    idempotencyKey: string,
    orderId?: string,
    guestToken?: string,
    promo?: { code: string; phoneE164?: string }
  ) =>
    api.post<Payment>(`/sessions/${sessionId}/payments`, {
      amount,
      method,
      idempotency_key: idempotencyKey,
      order_id: orderId ?? null,
      ...(promo?.code ? { promo_code: promo.code } : {}),
      ...(promo?.phoneE164 ? { phone_e164: promo.phoneE164 } : {}),
    }, { guestToken }),

  getBill: (sessionId: string, guestToken?: string) =>
    api.get<BillData>(`/sessions/${sessionId}/bill`, { guestToken }),

  // Payments awaiting staff confirmation for a branch (waiter settlement queue).
  listPendingForBranch: (branchId: number, staffToken: string) =>
    api.get<PendingPayment[]>(`/branches/${branchId}/payments`, { staffToken }),

  // Waiter/manager/owner confirms a cash/card collection: requires_staff_confirmation -> completed.
  settle: (paymentId: number, staffToken: string) =>
    api.patch<Payment>(`/payments/${paymentId}/settle`, {}, { staffToken }),
}
