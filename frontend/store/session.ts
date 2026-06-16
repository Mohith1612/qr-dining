import { create } from "zustand"
import type { Session, Participant, Payment } from "@/types/api"

interface SessionState {
  session: Session | null
  participant: Participant | null
  participants: Participant[]
  isHost: boolean
  completedPayment: Payment | null
  sessionExpiringAt: Date | null

  setSession: (session: Session, participant: Participant) => void
  setFromSnapshot: (session: Session, participants: Participant[]) => void
  addParticipant: (p: Participant) => void
  markClosed: () => void
  setCompletedPayment: (payment: Payment) => void
  setSessionExpiringAt: (at: Date | null) => void
  clear: () => void
}

export const useSessionStore = create<SessionState>((set, get) => ({
  session: null,
  participant: null,
  participants: [],
  isHost: false,
  completedPayment: null,
  sessionExpiringAt: null,

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

  setCompletedPayment(payment) {
    set({ completedPayment: payment })
  },

  setSessionExpiringAt(at) {
    set({ sessionExpiringAt: at })
  },

  markClosed() {
    set((s) => ({
      session: s.session ? { ...s.session, status: "closed" } : null,
      sessionExpiringAt: null,
    }))
  },

  clear() {
    set({ session: null, participant: null, participants: [], isHost: false, completedPayment: null, sessionExpiringAt: null })
  },
}))
