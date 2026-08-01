import { api } from "./client"
import type { CartItem, ModifierSnapshot } from "@/types/api"

interface BackendCartItem {
  id: number
  cart_id: number
  menu_item_id: number
  quantity: number
  selected_modifiers_json: ModifierSnapshot[] | null
  note: string
  item_name?: string
  item_price?: number
  image_url?: string | null
  is_available?: boolean
}

function mapCartItem(item: BackendCartItem): CartItem {
  return {
    ...item,
    session_id: "",
    participant_id: 0,
    added_at: "",
    selected_modifiers: item.selected_modifiers_json ?? [],
  } as CartItem
}

export const cartApi = {
  getCart: (sessionId: string, guestToken: string) =>
    api.get<{ Cart: { id: number; session_id: string; participant_id: number }; Items: BackendCartItem[] }>(
      `/sessions/${sessionId}/cart`,
      { guestToken }
    ).then(r => (r.Items ?? []).map(mapCartItem)),

  addItem: (
    sessionId: string,
    guestToken: string,
    menuItemId: number,
    quantity: number,
    modifierIds?: number[],
    note?: string
  ) =>
    api.post<BackendCartItem>(
      `/sessions/${sessionId}/cart/items`,
      { menu_item_id: menuItemId, quantity, modifier_ids: modifierIds, note },
      { guestToken }
    ).then(mapCartItem),

  removeItem: (sessionId: string, guestToken: string, itemId: number) =>
    api.delete<void>(`/sessions/${sessionId}/cart/items/${itemId}`, { guestToken }),
}
