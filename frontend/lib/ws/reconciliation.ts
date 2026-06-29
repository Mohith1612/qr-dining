import type { SessionSnapshot } from "@/types/api"
import { useSessionStore } from "@/store/session"
import { useOrdersStore } from "@/store/orders"
import { useAssistanceStore } from "@/store/assistance"
import { useCartStore } from "@/store/cart"
import { cartApi } from "@/lib/api/cart"

export function reconcileSnapshot(snapshot: SessionSnapshot): void {
  const session = snapshot.table_identifier
    ? { ...snapshot.session, table_identifier: snapshot.table_identifier }
    : snapshot.session
  useSessionStore.getState().setFromSnapshot(session, snapshot.participants)
  useOrdersStore.getState().setOrders(snapshot.orders)
  useAssistanceStore.getState().setRequests(snapshot.assistance)

  // The shared session cart is backend-authoritative and not carried in the
  // snapshot payload, so refetch it on reconnect to converge any edits made by
  // other participants while we were disconnected.
  const guestToken = sessionStorage.getItem("guest_access_token")
  if (guestToken) {
    cartApi
      .getCart(snapshot.session.id, guestToken)
      .then((items) => useCartStore.getState().setItems(items))
      .catch(() => {})
  }
}
