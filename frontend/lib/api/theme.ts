import { api } from "./client"
import type { ThemeConfig } from "@/lib/theme/applyTheme"

// Public (guest) theme resolution. GET /branches/:id/theme resolves the branch's
// restaurant theme; the backend bridges legacy settings_json.theme when no
// structured row exists, so this is always populated.
export const themeApi = {
  resolveForBranch: (branchId: number) =>
    api.get<{ theme: ThemeConfig }>(`/branches/${branchId}/theme`),
}
