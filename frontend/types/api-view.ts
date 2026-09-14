import type {
  ActiveAssistanceRequest,
  AssistanceRequest,
  CartItem,
  CartItemDetail,
  MenuCategory,
  MenuItem,
  MenuItemWithModifiers,
  Session,
  SessionWithTable,
} from "./api"

// Client projections compose generated wire types; they do not restate the API
// contract. A cart mutation returns CartItem, while a cart read enriches the row
// with menu display fields. The UI can hold either until its next cart refresh.
export type CartViewItem = Omit<CartItem, "selected_modifiers_json"> & {
  selected_modifiers: CartItem["selected_modifiers_json"]
  item_name?: CartItemDetail["item_name"]
  item_price?: CartItemDetail["item_price"]
  image_url?: CartItemDetail["image_url"]
  is_available?: CartItemDetail["is_available"]
}

// Full-menu reads always contain these nested fields, while create/update
// mutations return only the base sqlc row. Keeping them optional in transient
// client state models that distinction without weakening either wire schema.
export type MenuItemView = MenuItem &
  Partial<Pick<MenuItemWithModifiers, "modifiers">>
export type MenuCategoryView = MenuCategory & {
  items: MenuItemView[]
}

// The base Session response has no table label. Snapshot and staff-list
// responses add one, so the guest store carries it only after reconciliation.
export type SessionView = Session &
  Partial<Pick<SessionWithTable, "table_identifier">>

export type AssistanceRequestView = AssistanceRequest &
  Partial<
    Pick<ActiveAssistanceRequest, "table_identifier" | "session_number">
  >
