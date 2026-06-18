import { api } from "./client"
import type { Session, Participant, SessionSnapshot } from "@/types/api"

interface CreateSessionResponse {
  session: Session
  participant: Participant
  guest_access_token: string
}

interface WSTicketResponse {
  ticket: string
  expires_in: number
}

export const sessionsApi = {
  create: (tableId: number, displayName: string, deviceFingerprint?: string) =>
    api.post<CreateSessionResponse>("/sessions", {
      table_id: tableId,
      display_name: displayName,
      device_fingerprint: deviceFingerprint,
    }),

  get: (id: string, guestToken?: string) => api.get<Session>(`/sessions/${id}`, { guestToken }),

  join: (id: string, displayName: string, deviceFingerprint?: string) =>
    api.post<CreateSessionResponse>(`/sessions/${id}/join`, {
      display_name: displayName,
      device_fingerprint: deviceFingerprint,
    }),

  wsTicket: (id: string, guestToken: string) =>
    api.post<WSTicketResponse>(`/sessions/${id}/ws-ticket`, {}, { guestToken }),

  snapshot: (id: string, guestToken?: string) => api.get<SessionSnapshot>(`/sessions/${id}/snapshot`, { guestToken }),

  close: (id: string, participantId: number) =>
    api.delete<void>(`/sessions/${id}`, { participantId }),
}
