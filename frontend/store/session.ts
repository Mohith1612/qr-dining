import { create } from "zustand"
import type { Participant, Payment } from "@/types/api"
import type { SessionView as Session } from "@/types/api-view"

interface SessionState {
  session: Session | null
  participant: Participant | null
  participants: Participant[]
  isHost: boolean
  completedPayment: Payment | null
  // Set when staff withdraw a payment request. Screens showing "awaiting
  // confirmation" watch this to drop back out of the waiting state.
  cancelledPayment: Payment | null
  sessionExpiringAt: Date | null
  isReactivating: boolean

  setSession: (session: Session, participant: Participant) => void
  setFromSnapshot: (session: Session, participants: Participant[]) => void
  addParticipant: (p: Participant) => void
  applyHostChanged: (newHost: Participant) => void
  markClosed: () => void
  markPaused: () => void
  applyReactivated: (session: Session) => void
  setIsReactivating: (val: boolean) => void
  setCompletedPayment: (payment: Payment) => void
  applyPaymentCancelled: (payment: Payment, sessionStatus?: Session["status"]) => void
  setSessionExpiringAt: (at: Date | null) => void
  clear: () => void
}

export const useSessionStore = create<SessionState>((set, get) => ({
  session: null,
  participant: null,
  participants: [],
  isHost: false,
  completedPayment: null,
  cancelledPayment: null,
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

  // The snapshot is authoritative, so it must settle isReactivating too. This is
  // the function that actually runs on the automatic reactivation path: the
  // client showing the paused overlay is by definition the one without a
  // socket, so the SESSION_REACTIVATED event cannot reach it — the snapshot
  // reconcile is what does. Without this, a fixed overlay counting down to 0:00
  // sat over a live, connected session (F-08).
  setFromSnapshot(session, participants) {
    const self = get().participant
    const updated = self ? participants.find((p) => p.id === self.id) ?? self : self
    set({
      session,
      participants,
      participant: updated,
      isHost: updated?.is_host ?? false,
      isReactivating: session.status === "awaiting_reactivation",
    })
  },

  addParticipant(p) {
    set((s) => ({ participants: [...s.participants.filter((x) => x.id !== p.id), p] }))
  },

  applyHostChanged(newHost) {
    set((s) => ({
      participants: s.participants.map((p) => ({ ...p, is_host: p.id === newHost.id })),
      isHost: s.participant?.id === newHost.id,
      participant: s.participant
        ? { ...s.participant, is_host: s.participant.id === newHost.id }
        : s.participant,
      session: s.session
        ? { ...s.session, host_participant_id: newHost.id }
        : s.session,
    }))
  },

  markPaused() {
    set((s) => ({
      session: s.session ? { ...s.session, status: "awaiting_reactivation" } : null,
      isReactivating: true,
    }))
  },

  applyReactivated(session) {
    set({ session, isReactivating: false, sessionExpiringAt: null })
  },

  setIsReactivating(val) {
    set({ isReactivating: val })
  },

  setCompletedPayment(payment) {
    set({ completedPayment: payment })
  },

  // The server has already released the payment_pending freeze when no other
  // payment is outstanding; mirror the status it reports so the cart unlocks.
  applyPaymentCancelled(payment, sessionStatus) {
    set((s) => ({
      cancelledPayment: payment,
      session: s.session && sessionStatus ? { ...s.session, status: sessionStatus } : s.session,
    }))
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
    set({ session: null, participant: null, participants: [], isHost: false, completedPayment: null, cancelledPayment: null, sessionExpiringAt: null, isReactivating: false })
  },
}))
