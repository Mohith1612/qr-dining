import { api } from "./client"
import type { Payment, PaymentMethod } from "@/types/api"

export const paymentsApi = {
  initiate: (
    sessionId: string,
    amount: number,
    method: PaymentMethod,
    orderId?: string
  ) =>
    api.post<Payment>(`/sessions/${sessionId}/payments`, {
      amount,
      method,
      order_id: orderId ?? null,
    }),
}
