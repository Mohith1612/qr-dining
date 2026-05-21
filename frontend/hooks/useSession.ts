import { useSessionStore } from "@/store/session"

export function useSession() {
  const session = useSessionStore((s) => s.session)
  const participant = useSessionStore((s) => s.participant)
  const participants = useSessionStore((s) => s.participants)
  const isHost = useSessionStore((s) => s.isHost)
  return { session, participant, participants, isHost }
}
