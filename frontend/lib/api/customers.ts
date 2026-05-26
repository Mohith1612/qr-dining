import { api } from "./client"

export const customersApi = {
  link: (sessionId: string, phone: string, displayName: string, participantId: number) =>
    api.post<{ ok: boolean }>(
      `/sessions/${sessionId}/customer`,
      { phone, display_name: displayName },
      { participantId }
    ),
}
