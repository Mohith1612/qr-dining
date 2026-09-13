"use client"

import { useCallback, useEffect, useRef } from "react"
import { WSConnection } from "@/lib/ws/connection"
import { useSessionStore } from "@/store/session"
import { useCartStore } from "@/store/cart"
import { useMenuStore } from "@/store/menu"
import { useOrdersStore } from "@/store/orders"
import { useAssistanceStore } from "@/store/assistance"
import { useWsStore } from "@/store/ws"
import { cartApi } from "@/lib/api/cart"
import { toast } from "sonner"
import type { WSEventHandlerMap } from "@/types/ws"
import type { Participant, Order, AssistanceRequest, Payment, Session } from "@/types/api"
import { track } from "@/lib/product-analytics/events"
import { registerGuestSessionProps, unregisterGuestSessionProps } from "@/lib/product-analytics/identity"

export function useWebSocket(sessionId: string) {
  const connRef = useRef<WSConnection | null>(null)

  useEffect(() => {
    const handlers: WSEventHandlerMap = {
      SESSION_CLOSED: () => {
        if (useSessionStore.getState().isHost) {
          track("session_ended", { session_id: sessionId })
        }
        useSessionStore.getState().markClosed()
        unregisterGuestSessionProps()
      },

      SESSION_REACTIVATED: () => {
        useSessionStore.getState().setIsReactivating(false)
      },

      SESSION_EXPIRING_SOON: (payload) => {
        const p = payload as { expires_at?: string } | null
        const expiresAt = p?.expires_at
          ? new Date(p.expires_at)
          : new Date(Date.now() + 15 * 60 * 1000)
        useSessionStore.getState().setSessionExpiringAt(expiresAt)
      },

      PAYMENT_COMPLETED: (payload) => {
        const payment = payload as Payment
        if (useSessionStore.getState().isHost) {
          track("payment_completed", {
            method: payment.method,
            amount: Number(payment.amount),
          })
        }
        useSessionStore.getState().setCompletedPayment(payment)
      },

      // Staff withdrew the payment request — the bill is unpaid again and the
      // cart is unfrozen. Without this the guest sits on "Awaiting confirmation"
      // forever, because nothing else ever contradicts the initiate response.
      PAYMENT_CANCELLED: (payload) => {
        const p = payload as { payment?: Payment; session_status?: Session["status"] } | null
        if (!p?.payment) return
        useSessionStore.getState().applyPaymentCancelled(p.payment, p.session_status)
        toast.info("Your server cancelled the payment request. You can keep ordering or try paying again.")
      },

      // An admin toggled a menu item. Reconcile the menu in realtime; if a
      // now-unavailable item is in the cart, flag it for the guest. This is an
      // informational update — never a connection/session error.
      MENU_ITEM_AVAILABILITY_CHANGED: (payload) => {
        const p = payload as { item_id?: number; is_available?: boolean } | null
        if (!p || typeof p.item_id !== "number" || typeof p.is_available !== "boolean") return
        useMenuStore.getState().setItemAvailability(p.item_id, p.is_available)
        if (!p.is_available) {
          const inCart = useCartStore.getState().items.some((i) => i.menu_item_id === p.item_id)
          if (inCart) {
            toast.warning("An item in your cart is no longer available. Please review your cart before ordering.")
          }
        }
      },

      PARTICIPANT_JOINED: (payload) => {
        const p = payload as Participant
        useSessionStore.getState().addParticipant(p)
      },

      PARTICIPANT_LEFT: (payload) => {
        const p = payload as Participant
        useSessionStore.setState((s) => ({
          participants: s.participants.filter((x) => x.id !== p.id),
        }))
      },

      HOST_CHANGED: (payload) => {
        const newHost = payload as Participant
        const wasHost = useSessionStore.getState().isHost
        useSessionStore.getState().applyHostChanged(newHost)
        const state = useSessionStore.getState()
        if (state.session && state.participant) {
          registerGuestSessionProps({
            sessionId: state.session.id,
            participantId: state.participant.id,
            isHost: state.isHost,
            tableId: state.session.table_id,
          })
        }
        if (state.isHost && !wasHost) {
          toast.info("You're now the table host — you can send orders and request the bill.")
        }
      },

      CART_UPDATED: async () => {
        try {
          const guestToken = sessionStorage.getItem("guest_access_token")
          if (!guestToken) return
          const items = await cartApi.getCart(sessionId, guestToken)
          useCartStore.getState().setItems(items)
        } catch {}
      },

      ORDER_PLACED: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().addOrder(order)
      },

      // Order status events are published wrapped as { order: {...} } (order.go
      // publishOrderStatusEvent), unlike ORDER_PLACED which is top-level. Unwrap
      // defensively so the live tracker advances without a page refresh (F-6).
      ORDER_CONFIRMED: (payload) => {
        const order = ((payload as { order?: Order })?.order ?? payload) as Order
        if (!order?.id) return
        useOrdersStore.getState().updateStatus(order.id, "confirmed")
      },

      ORDER_PREPARING: (payload) => {
        const order = ((payload as { order?: Order })?.order ?? payload) as Order
        if (!order?.id) return
        useOrdersStore.getState().updateStatus(order.id, "preparing")
      },

      ORDER_READY: (payload) => {
        const order = ((payload as { order?: Order })?.order ?? payload) as Order
        if (!order?.id) return
        useOrdersStore.getState().updateStatus(order.id, "ready")
      },

      ORDER_SERVED: (payload) => {
        const order = ((payload as { order?: Order })?.order ?? payload) as Order
        if (!order?.id) return
        useOrdersStore.getState().updateStatus(order.id, "served")
      },

      ORDER_CANCELLED: (payload) => {
        const order = ((payload as { order?: Order })?.order ?? payload) as Order
        if (!order?.id) return
        useOrdersStore.getState().updateStatus(order.id, "cancelled")
      },

      ASSISTANCE_REQUESTED: (payload) => {
        const req = payload as AssistanceRequest
        useAssistanceStore.getState().addRequest(req)
      },

      ASSISTANCE_ACKNOWLEDGED: (payload) => {
        const req = payload as AssistanceRequest
        useAssistanceStore.getState().updateStatus(req.id, "acknowledged")
      },

      ASSISTANCE_RESOLVED: (payload) => {
        const req = payload as AssistanceRequest
        useAssistanceStore.getState().updateStatus(req.id, "resolved")
      },
    }

    const conn = new WSConnection(sessionId, handlers)
    connRef.current = conn
    conn.connect()

    return () => {
      conn.disconnect()
      connRef.current = null
    }
  }, [sessionId])

  const status = useWsStore((s) => s.status)
  const attempt = useWsStore((s) => s.attempt)
  const retry = useCallback(() => {
    connRef.current?.retry()
  }, [])
  return { status, attempt, retry }
}
