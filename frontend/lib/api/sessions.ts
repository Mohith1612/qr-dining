import { api } from "./client"
import type { Session, Participant, SessionSnapshot } from "@/types/api"

interface CreateSessionResponse {
  session: Session
  participant: Participant
}

export const sessionsApi = {
  create: (tableId: number, displayName: string, deviceFingerprint?: string) =>
    api.post<CreateSessionResponse>("/sessions", {
      table_id: tableId,
      display_name: displayName,
      device_fingerprint: deviceFingerprint,
    }),

  get: (id: string) => api.get<Session>(`/sessions/${id}`),

  join: (id: string, displayName: string, deviceFingerprint?: string) =>
    api.post<Participant>(`/sessions/${id}/join`, {
      display_name: displayName,
      device_fingerprint: deviceFingerprint,
    }),

  snapshot: (id: string) => api.get<SessionSnapshot>(`/sessions/${id}/snapshot`),

  close: (id: string, participantId: number) =>
    api.delete<void>(`/sessions/${id}`, { participantId }),
}
