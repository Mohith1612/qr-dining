import { api } from "./client"
import type { MenuCategory } from "@/types/api"

interface TableByQrResponse {
  table_id: number
  branch_id: number
  label?: string
}

export const menuApi = {
  getMenu: (branchId: number) =>
    api.get<{ branch_id: number; categories: MenuCategory[] }>(`/branches/${branchId}/menu`)
      .then(r => r.categories),

  resolveQrToken: (token: string) =>
    api.get<{ id: number; branch_id: number; identifier: string }>(`/tables/by-qr/${token}`)
      .then(r => ({ table_id: r.id, branch_id: r.branch_id, label: `Table ${r.identifier}` })),
}
