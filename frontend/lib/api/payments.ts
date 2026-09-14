import { api } from "./client"
import type { Payment, PendingPayment, PaymentMethod, Bill as BillData } from "@/types/api"

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
  // The backend only accepts the two non-terminal statuses it can serve here:
  // requires_staff_confirmation (default) and provider_pending.
  listPendingForBranch: (
    branchId: number,
    staffToken: string,
    status?: "requires_staff_confirmation" | "provider_pending"
  ) =>
    api.get<PendingPayment[]>(
      `/branches/${branchId}/payments${status ? `?status=${status}` : ""}`,
      { staffToken }
    ),

  // Waiter/manager/owner confirms a cash/card collection: requires_staff_confirmation -> completed.
  settle: (paymentId: number, staffToken: string) =>
    api.patch<Payment>(`/payments/${paymentId}/settle`, {}, { staffToken }),

  // Waiter/manager/owner withdraws a bill request the guest no longer wants.
  // The payment moves to cancelled and the session unfreezes back to active, so
  // the table can order again. `reason` is required and goes to the audit log.
  cancel: (paymentId: number, reason: string, staffToken: string) =>
    api.patch<Payment>(`/payments/${paymentId}/cancel`, { reason }, { staffToken }),
}
