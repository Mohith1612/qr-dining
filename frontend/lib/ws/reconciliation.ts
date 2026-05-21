import type { SessionSnapshot } from "@/types/api"
import { useSessionStore } from "@/store/session"
import { useOrdersStore } from "@/store/orders"
import { useAssistanceStore } from "@/store/assistance"

export function reconcileSnapshot(snapshot: SessionSnapshot): void {
  useSessionStore.getState().setFromSnapshot(snapshot.session, snapshot.participants)
  useOrdersStore.getState().setOrders(snapshot.orders)
  useAssistanceStore.getState().setRequests(snapshot.assistance)
}
