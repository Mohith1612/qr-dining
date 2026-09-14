import { api } from "./client"
import type { FullMenu } from "@/types/api"

export const menuApi = {
  getMenu: (branchId: number) =>
    api.get<FullMenu>(`/branches/${branchId}/menu`)
      .then(r => ({ featured: r.featured, categories: r.categories })),

  resolveQrToken: (token: string) =>
    api.get<{ id: number; branch_id: number; identifier: string; session_id?: string; branch_theme?: string }>(`/tables/by-qr/${token}`)
      .then(r => ({ table_id: r.id, branch_id: r.branch_id, label: r.identifier, session_id: r.session_id, branch_theme: r.branch_theme })),
}
