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
  getCart: (sessionId: string, participantId: number) =>
    api.get<{ Cart: { id: number; session_id: string; participant_id: number }; Items: BackendCartItem[] }>(
      `/sessions/${sessionId}/cart`,
      { participantId }
    ).then(r => (r.Items ?? []).map(mapCartItem)),

  addItem: (
    sessionId: string,
    participantId: number,
    menuItemId: number,
    quantity: number,
    modifierIds?: number[],
    note?: string
  ) =>
    api.post<BackendCartItem>(
      `/sessions/${sessionId}/cart/items`,
      { menu_item_id: menuItemId, quantity, modifier_ids: modifierIds, note },
      { participantId }
    ).then(mapCartItem),

  removeItem: (sessionId: string, participantId: number, itemId: number) =>
    api.delete<void>(`/sessions/${sessionId}/cart/items/${itemId}`, { participantId }),
}
