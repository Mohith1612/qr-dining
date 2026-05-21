import { api } from "./client"
import type { MenuCategory } from "@/types/api"

interface TableByQrResponse {
  table_id: number
  branch_id: number
  label?: string
}

export const menuApi = {
  getMenu: (branchId: number) =>
    api.get<MenuCategory[]>(`/branches/${branchId}/menu`),

  resolveQrToken: (token: string) =>
    api.get<TableByQrResponse>(`/tables/by-qr/${token}`),
}
