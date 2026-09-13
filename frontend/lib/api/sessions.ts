import { api } from "./client"
import type { Session, Participant, SessionSnapshot, ForceCloseResult } from "@/types/api"

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
  create: (tableId: number, displayName: string, phoneE164?: string, deviceFingerprint?: string) =>
    api.post<CreateSessionResponse>("/sessions", {
      table_id: tableId,
      display_name: displayName,
      phone_e164: phoneE164 || undefined,
      device_fingerprint: deviceFingerprint,
    }),

  get: (id: string, guestToken?: string) => api.get<Session>(`/sessions/${id}`, { guestToken }),

  join: (id: string, displayName: string, phoneE164?: string, deviceFingerprint?: string) =>
    api.post<CreateSessionResponse>(`/sessions/${id}/join`, {
      display_name: displayName,
      phone_e164: phoneE164 || undefined,
      device_fingerprint: deviceFingerprint,
    }),

  wsTicket: (id: string, guestToken: string, since?: number) =>
    api.post<WSTicketResponse>(
      `/sessions/${id}/ws-ticket`,
      since && since > 0 ? { since } : {},
      { guestToken }
    ),

  snapshot: (id: string, guestToken?: string, lastSequence?: number) => {
    const query = lastSequence && lastSequence > 0 ? `?last_sequence=${lastSequence}` : ""
    return api.get<SessionSnapshot>(`/sessions/${id}/snapshot${query}`, { guestToken })
  },

  reactivate: (id: string, guestToken: string) =>
    api.post<Session>(`/sessions/${id}/reactivate`, {}, { guestToken }),

  // Host hands the host role to another participant. The HOST_CHANGED WS event
  // updates every client (badges + host-only controls).
  transferHost: (id: string, participantId: number, guestToken: string) =>
    api.post<{ ok: boolean; host_participant_id: number }>(
      `/sessions/${id}/host`,
      { participant_id: participantId },
      { guestToken }
    ),

  close: (id: string, guestToken: string) =>
    api.delete<void>(`/sessions/${id}`, { guestToken }),

  // Manager/owner ends a table the guests never closed — walked out, or something
  // happened outside the app. Closes the session, releases the table, revokes
  // every guest credential and cancels any outstanding payment. `reason` is
  // required and goes to the audit log.
  forceClose: (id: string, reason: string, staffToken: string) =>
    api.post<ForceCloseResult>(`/sessions/${id}/force-close`, { reason }, { staffToken }),
}
