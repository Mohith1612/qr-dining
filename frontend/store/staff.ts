import { create } from "zustand"
import { persist } from "zustand/middleware"
import type { StaffRole } from "@/types/api"

interface StaffState {
  token: string | null
  staffId: number | null
  branchId: number | null
  role: StaffRole | null
  _hydrated: boolean

  setAuth: (token: string, staffId: number, branchId: number, role: StaffRole) => void
  clear: () => void
  _setHydrated: () => void
}

export const useStaffStore = create<StaffState>()(
  persist(
    (set) => ({
      token: null,
      staffId: null,
      branchId: null,
      role: null,
      _hydrated: false,

      setAuth(token, staffId, branchId, role) {
        set({ token, staffId, branchId, role })
      },

      clear() {
        set({ token: null, staffId: null, branchId: null, role: null })
      },

      _setHydrated() {
        set({ _hydrated: true })
      },
    }),
    {
      name: "staff-auth",
      onRehydrateStorage: () => (state) => {
        state?._setHydrated()
      },
    }
  )
)
