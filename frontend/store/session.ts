import { create } from "zustand"
import type { Session, Participant, Payment } from "@/types/api"

interface SessionState {
  session: Session | null
  participant: Participant | null
  participants: Participant[]
  isHost: boolean
  completedPayment: Payment | null
  sessionExpiringAt: Date | null
  isReactivating: boolean

  setSession: (session: Session, participant: Participant) => void
  setFromSnapshot: (session: Session, participants: Participant[]) => void
  addParticipant: (p: Participant) => void
  markClosed: () => void
  markPaused: () => void
  setIsReactivating: (val: boolean) => void
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
  isReactivating: false,

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

  markPaused() {
    set((s) => ({
      session: s.session ? { ...s.session, status: "awaiting_reactivation" } : null,
      isReactivating: true,
    }))
  },

  setIsReactivating(val) {
    set({ isReactivating: val })
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
      isReactivating: false,
    }))
  },

  clear() {
    set({ session: null, participant: null, participants: [], isHost: false, completedPayment: null, sessionExpiringAt: null, isReactivating: false })
  },
}))
