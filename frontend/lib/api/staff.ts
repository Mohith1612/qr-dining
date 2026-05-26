import { api } from "./client"
import type {
  StaffSession,
  Staff,
  StaffRole,
  Session,
  Order,
  MenuCategory,
  MenuItem,
  DietaryFlag,
  ItemBadge,
} from "@/types/api"

export const staffApi = {
  auth: (branchId: number, pin: string) =>
    api.post<StaffSession>("/staff/auth", { branch_id: branchId, pin }),

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

  deactivate: (staffId: number, staffToken: string) =>
    api.patch<void>(`/staff/${staffId}/deactivate`, {}, { staffToken }),

  getActiveSessions: (branchId: number, staffToken: string) =>
    api.get<Session[]>(`/branches/${branchId}/sessions/active`, { staffToken }),

  getActiveOrders: (branchId: number, staffToken: string) =>
    api.get<Order[]>(`/branches/${branchId}/orders/active`, { staffToken }),

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
    opts?: { description?: string; position?: number; is_available?: boolean }
  ) =>
    api.post<MenuItem>(
      `/branches/${branchId}/menu/items`,
      { category_id: categoryId, name, price, ...opts },
      { staffToken }
    ),

  updateMenuItem: (
    itemId: number,
    branchId: number,
    data: { name: string; price: number; description?: string; position?: number; dietary_flags?: DietaryFlag[]; item_badges?: ItemBadge[]; spice_level?: number },
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
    data: { session_timeout_minutes?: number },
    staffToken: string
  ) =>
    api.patch<void>(`/branches/${branchId}`, data, { staffToken }),

  getBranch: (branchId: number, staffToken: string) =>
    api.get<{ id: number; session_timeout_minutes: number }>(`/branches/${branchId}`, { staffToken }),
}
