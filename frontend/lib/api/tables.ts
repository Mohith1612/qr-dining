import { api } from "./client"
import type { Table } from "@/types/api"

export const tablesApi = {
  list: (branchId: number, staffToken: string) =>
    api.get<{ tables: Table[] }>(`/branches/${branchId}/tables`, { staffToken })
      .then(r => r.tables),

  create: (branchId: number, data: { identifier: string; capacity?: number }, staffToken: string) =>
    api.post<Table>(`/branches/${branchId}/tables`, data, { staffToken }),

  refreshQR: (tableId: number, staffToken: string) =>
    api.patch<Table>(`/tables/${tableId}/qr-refresh`, {}, { staffToken }),
}
