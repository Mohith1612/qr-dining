import { api } from "./client"
import type { AssistanceRequest, AssistanceType } from "@/types/api"

export const assistanceApi = {
  request: (
    sessionId: string,
    tableId: number,
    guestToken: string,
    type: AssistanceType = "waiter"
  ) =>
    api.post<AssistanceRequest>(`/sessions/${sessionId}/assist`, {
      table_id: tableId,
      type,
    }, { guestToken }),

  acknowledge: (id: number, staffToken: string) =>
    api.patch<AssistanceRequest>(`/assist/${id}/ack`, {}, { staffToken }),

  resolve: (id: number, staffToken: string) =>
    api.patch<AssistanceRequest>(`/assist/${id}/resolve`, {}, { staffToken }),

  getActive: (branchId: number, staffToken: string) =>
    api.get<AssistanceRequest[]>(`/branches/${branchId}/assist/active`, { staffToken }),
}
