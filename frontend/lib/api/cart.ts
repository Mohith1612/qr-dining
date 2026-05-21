import { api } from "./client"
import type { CartItem } from "@/types/api"

export const cartApi = {
  getCart: (sessionId: string, participantId: number) =>
    api.get<CartItem[]>(`/sessions/${sessionId}/cart`, { participantId }),

  addItem: (
    sessionId: string,
    participantId: number,
    menuItemId: number,
    quantity: number,
    modifierIds?: number[],
    note?: string
  ) =>
    api.post<CartItem>(
      `/sessions/${sessionId}/cart/items`,
      { menu_item_id: menuItemId, quantity, modifier_ids: modifierIds, note },
      { participantId }
    ),

  removeItem: (sessionId: string, participantId: number, itemId: number) =>
    api.delete<void>(`/sessions/${sessionId}/cart/items/${itemId}`, { participantId }),
}
