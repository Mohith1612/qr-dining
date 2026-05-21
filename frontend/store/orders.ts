import { create } from "zustand"
import type { Order, OrderStatus } from "@/types/api"

interface OrdersState {
  orders: Order[]

  setOrders: (orders: Order[]) => void
  addOrder: (order: Order) => void
  updateStatus: (orderId: string, status: OrderStatus) => void
}

export const useOrdersStore = create<OrdersState>((set) => ({
  orders: [],

  setOrders(orders) {
    set({ orders })
  },

  addOrder(order) {
    set((s) => {
      const exists = s.orders.some((o) => o.id === order.id)
      if (exists) return s
      return { orders: [order, ...s.orders] }
    })
  },

  updateStatus(orderId, status) {
    set((s) => ({
      orders: s.orders.map((o) =>
        o.id === orderId ? { ...o, status, updated_at: new Date().toISOString() } : o
      ),
    }))
  },
}))
