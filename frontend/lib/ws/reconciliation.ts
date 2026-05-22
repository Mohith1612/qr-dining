import type { SessionSnapshot } from "@/types/api"
import { useSessionStore } from "@/store/session"
import { useOrdersStore } from "@/store/orders"
import { useAssistanceStore } from "@/store/assistance"

export function reconcileSnapshot(snapshot: SessionSnapshot): void {
  const session = snapshot.table_identifier
    ? { ...snapshot.session, table_identifier: snapshot.table_identifier }
    : snapshot.session
  useSessionStore.getState().setFromSnapshot(session, snapshot.participants)
  useOrdersStore.getState().setOrders(snapshot.orders)
  useAssistanceStore.getState().setRequests(snapshot.assistance)
}
