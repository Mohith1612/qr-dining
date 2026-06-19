import { api } from "./client"
import type { Payment, PaymentMethod, BillData } from "@/types/api"

export const paymentsApi = {
  initiate: (
    sessionId: string,
    amount: number,
    method: PaymentMethod,
    idempotencyKey: string,
    orderId?: string,
    participantId?: number
  ) =>
    api.post<Payment>(`/sessions/${sessionId}/payments`, {
      amount,
      method,
      idempotency_key: idempotencyKey,
      order_id: orderId ?? null,
    }, { participantId }),

  getBill: (sessionId: string, participantId?: number) =>
    api.get<BillData>(`/sessions/${sessionId}/bill`, { participantId }),
}
