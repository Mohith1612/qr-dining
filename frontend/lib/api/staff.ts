import { api } from "./client"
import type {
  StaffSession,
  Staff,
  StaffRosterMember,
  StaffRole,
  Session,
  KitchenOrder,
  MenuCategory,
  MenuItem,
  ItemModifier,
  DietaryFlag,
  ItemBadge,
} from "@/types/api"
import type { CollateralConfig } from "@/types/collateral"

export const staffApi = {
  auth: (branchCode: string, staffCode: string, pin: string) =>
    api.post<StaffSession>("/staff/auth", { branch_code: branchCode, staff_code: staffCode, pin }),

  logout: (staffToken: string) =>
    api.post<void>("/staff/logout", {}, { staffToken }),

  createStaff: (
    branchId: number,
    name: string,
    role: StaffRole,
    pin: string,
    staffToken: string
  ) =>
    api.post<Staff>(
      `/branches/${branchId}/staff`,
      { name, role, pin },
      { staffToken }
    ),

  rotatePin: (
    staffId: number,
    currentPin: string,
    newPin: string,
    staffToken: string
  ) =>
    api.patch<void>(
      `/staff/${staffId}/pin`,
      { current_pin: currentPin, new_pin: newPin },
      { staffToken }
    ),

  // Owner/manager forgotten-PIN reset (no current_pin). Manager may reset only
  // waiter/kitchen; owner may reset anyone in branch; never self.
  resetPin: (staffId: number, newPin: string, staffToken: string) =>
    api.post<void>(`/staff/${staffId}/pin/reset`, { new_pin: newPin }, { staffToken }),

  listStaff: (branchId: number, staffToken: string) =>
    api.get<{ staff: StaffRosterMember[] }>(`/branches/${branchId}/staff`, { staffToken }),

  deactivate: (staffId: number, staffToken: string) =>
    api.patch<void>(`/staff/${staffId}/deactivate`, {}, { staffToken }),

  getActiveSessions: (branchId: number, staffToken: string) =>
    api.get<Session[]>(`/branches/${branchId}/sessions/active`, { staffToken }),

  getActiveOrders: (branchId: number, staffToken: string) =>
    api.get<KitchenOrder[]>(`/branches/${branchId}/orders/active`, { staffToken }),

  createCategory: (
    branchId: number,
    name: string,
    position: number,
    staffToken: string
  ) =>
    api.post<MenuCategory>(
      `/branches/${branchId}/menu/categories`,
      { name, position },
      { staffToken }
    ),

  createMenuItem: (
    branchId: number,
    categoryId: number,
    name: string,
    price: number,
    staffToken: string,
    opts?: {
      description?: string
      position?: number
      is_available?: boolean
      dietary_flags?: DietaryFlag[]
      item_badges?: ItemBadge[]
      spice_level?: number
    }
  ) =>
    api.post<MenuItem>(
      `/branches/${branchId}/menu/items`,
      { category_id: categoryId, name, price, ...opts },
      { staffToken }
    ),

  updateMenuItem: (
    itemId: number,
    branchId: number,
    data: { name: string; price: number; description?: string; position?: number; dietary_flags?: DietaryFlag[]; item_badges?: ItemBadge[]; spice_level?: number; category_id?: number },
    staffToken: string
  ) =>
    api.patch<MenuItem>(`/menu/items/${itemId}`, { ...data, branch_id: branchId }, { staffToken }),

  toggleAvailability: (
    itemId: number,
    branchId: number,
    available: boolean,
    staffToken: string
  ) =>
    api.patch<void>(
      `/menu/items/${itemId}/availability`,
      { available, branch_id: branchId },
      { staffToken }
    ),

  toggleFeatured: (
    itemId: number,
    branchId: number,
    isFeatured: boolean,
    sortOrder: number,
    staffToken: string
  ) =>
    api.patch<void>(
      `/menu/items/${itemId}/featured`,
      { is_featured: isFeatured, featured_sort_order: sortOrder, branch_id: branchId },
      { staffToken }
    ),

  updateBranch: (
    branchId: number,
    data: {
      session_timeout_minutes?: number
      order_prefix?: string
      theme?: string
      tax_rate?: number
      service_charge_rate?: number
      include_tax_in_price?: boolean
    },
    staffToken: string
  ) =>
    api.patch<void>(`/branches/${branchId}`, data, { staffToken }),

  getBranch: (branchId: number, staffToken: string) =>
    api.get<{
      id: number
      name: string
      organization_id: number
      restaurant_id: number
      branch_code: string
      status: string
      support_metadata: Record<string, unknown>
      session_timeout_minutes: number
      order_prefix: string
      theme: string
      tax_rate: number
      service_charge_rate: number
      include_tax_in_price: boolean
      customer_memory_enabled: boolean
      restaurant_name: string
      logo_url: string
    }>(`/branches/${branchId}`, { staffToken }),

  getCollateral: (branchId: number, staffToken: string) =>
    api.get<{ collateral: CollateralConfig; formats: string[] }>(
      `/branches/${branchId}/collateral`,
      { staffToken }
    ),

  setCollateral: (branchId: number, config: CollateralConfig, staffToken: string) =>
    api.put<{ collateral: CollateralConfig }>(`/branches/${branchId}/collateral`, config, { staffToken }),

  getAdminMenu: (branchId: number, staffToken: string) =>
    api.get<{ branch_id: number; categories: MenuCategory[] }>(`/branches/${branchId}/menu/full`, { staffToken }),

  deleteMenuItem: (itemId: number, branchId: number, staffToken: string) =>
    api.delete<void>(`/menu/items/${itemId}?branch_id=${branchId}`, { staffToken }),

  deleteCategory: (categoryId: number, branchId: number, staffToken: string) =>
    api.delete<void>(`/menu/categories/${categoryId}?branch_id=${branchId}`, { staffToken }),

  updateCategory: (
    categoryId: number,
    branchId: number,
    data: { name: string; position: number; is_active: boolean },
    staffToken: string
  ) =>
    api.patch<MenuCategory>(`/menu/categories/${categoryId}`, { ...data, branch_id: branchId }, { staffToken }),

  addModifier: (
    itemId: number,
    branchId: number,
    data: { name: string; price_delta: number; is_required: boolean; modifier_group: string; single_select?: boolean },
    staffToken: string
  ) =>
    api.post<ItemModifier>(`/menu/items/${itemId}/modifiers`, { ...data, branch_id: branchId }, { staffToken }),

  updateModifier: (
    modifierId: number,
    branchId: number,
    data: { name: string; price_delta: number; is_required: boolean; modifier_group: string; single_select?: boolean },
    staffToken: string
  ) =>
    api.patch<ItemModifier>(`/menu/modifiers/${modifierId}`, { ...data, branch_id: branchId }, { staffToken }),

  deleteModifier: (modifierId: number, branchId: number, staffToken: string) =>
    api.delete<void>(`/menu/modifiers/${modifierId}?branch_id=${branchId}`, { staffToken }),
}
