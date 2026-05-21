import { create } from "zustand"
import { persist } from "zustand/middleware"
import type { StaffRole } from "@/types/api"

interface StaffState {
  token: string | null
  staffId: number | null
  branchId: number | null
  role: StaffRole | null

  setAuth: (token: string, staffId: number, branchId: number, role: StaffRole) => void
  clear: () => void
}

export const useStaffStore = create<StaffState>()(
  persist(
    (set) => ({
      token: null,
      staffId: null,
      branchId: null,
      role: null,

      setAuth(token, staffId, branchId, role) {
        set({ token, staffId, branchId, role })
      },

      clear() {
        set({ token: null, staffId: null, branchId: null, role: null })
      },
    }),
    { name: "staff-auth" }
  )
)
