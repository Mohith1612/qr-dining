import { create } from "zustand"
import type { Session, Participant } from "@/types/api"

interface SessionState {
  session: Session | null
  participant: Participant | null
  participants: Participant[]
  isHost: boolean

  setSession: (session: Session, participant: Participant) => void
  setFromSnapshot: (session: Session, participants: Participant[]) => void
  addParticipant: (p: Participant) => void
  markClosed: () => void
  clear: () => void
}

export const useSessionStore = create<SessionState>((set, get) => ({
  session: null,
  participant: null,
  participants: [],
  isHost: false,

  setSession(session, participant) {
    set({
      session,
      participant,
      isHost: participant.is_host,
      participants: [participant],
    })
  },

  setFromSnapshot(session, participants) {
    const self = get().participant
    const updated = self ? participants.find((p) => p.id === self.id) ?? self : self
    set({
      session,
      participants,
      participant: updated,
      isHost: updated?.is_host ?? false,
    })
  },

  addParticipant(p) {
    set((s) => ({ participants: [...s.participants.filter((x) => x.id !== p.id), p] }))
  },

  markClosed() {
    set((s) => ({
      session: s.session ? { ...s.session, status: "closed" } : null,
    }))
  },

  clear() {
    set({ session: null, participant: null, participants: [], isHost: false })
  },
}))
