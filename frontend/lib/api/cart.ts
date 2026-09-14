import { api } from "./client"
import type { CartItem, CartItemDetail, CartResponse } from "@/types/api"
import type { CartViewItem } from "@/types/api-view"

function mapCartItem(item: CartItem | CartItemDetail): CartViewItem {
  return {
    ...item,
    selected_modifiers: item.selected_modifiers_json,
  }
}

export const cartApi = {
  getCart: (sessionId: string, guestToken: string) =>
    api.get<CartResponse>(
      `/sessions/${sessionId}/cart`,
      { guestToken }
    ).then(r => r.items.map(mapCartItem)),

  addItem: (
    sessionId: string,
    guestToken: string,
    menuItemId: number,
    quantity: number,
    modifierIds?: number[],
    note?: string
  ) =>
    api.post<CartItem>(
      `/sessions/${sessionId}/cart/items`,
      { menu_item_id: menuItemId, quantity, modifier_ids: modifierIds, note },
      { guestToken }
    ).then(mapCartItem),

  removeItem: (sessionId: string, guestToken: string, itemId: number) =>
    api.delete<void>(`/sessions/${sessionId}/cart/items/${itemId}`, { guestToken }),
}
