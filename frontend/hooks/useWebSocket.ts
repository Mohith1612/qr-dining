"use client"

import { useEffect, useRef } from "react"
import { WSConnection } from "@/lib/ws/connection"
import { reconcileSnapshot } from "@/lib/ws/reconciliation"
import { useSessionStore } from "@/store/session"
import { useCartStore } from "@/store/cart"
import { useOrdersStore } from "@/store/orders"
import { useAssistanceStore } from "@/store/assistance"
import { useWsStore } from "@/store/ws"
import { cartApi } from "@/lib/api/cart"
import type { WSEventHandlerMap } from "@/types/ws"
import type { Participant, Order, AssistanceRequest, SessionSnapshot } from "@/types/api"

export function useWebSocket(sessionId: string, participantId: number) {
  const connRef = useRef<WSConnection | null>(null)

  useEffect(() => {
    const handlers: WSEventHandlerMap = {
      SESSION_CLOSED: () => {
        useSessionStore.getState().markClosed()
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

      CART_UPDATED: async () => {
        try {
          const items = await cartApi.getCart(sessionId, participantId)
          useCartStore.getState().setItems(items)
        } catch {}
      },

      ORDER_PLACED: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().addOrder(order)
      },

      ORDER_CONFIRMED: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().updateStatus(order.id, "confirmed")
      },

      ORDER_PREPARING: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().updateStatus(order.id, "preparing")
      },

      ORDER_READY: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().updateStatus(order.id, "ready")
      },

      ORDER_SERVED: (payload) => {
        const order = payload as Order
        useOrdersStore.getState().updateStatus(order.id, "served")
      },

      ORDER_CANCELLED: (payload) => {
        const order = payload as Order
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

    const conn = new WSConnection(sessionId, participantId, handlers)
    connRef.current = conn
    conn.connect()

    return () => {
      conn.disconnect()
      connRef.current = null
    }
  }, [sessionId, participantId])

  const status = useWsStore((s) => s.status)
  const attempt = useWsStore((s) => s.attempt)
  return { status, attempt }
}
