import { create } from "zustand"
import { persist, createJSONStorage } from "zustand/middleware"
import type { PlatformRole } from "@/types/platform"

// Platform operator session. Separate trust domain from staff/guest — its own
// sessionStorage key and store; never shares the staff token.
interface PlatformState {
  token: string | null
  platformUserId: number | null
  email: string | null
  displayName: string | null
  roles: PlatformRole[]
  expiresAt: string | null
  _hydrated: boolean

  setSession: (s: {
    token: string
    platformUserId: number
    email: string
    displayName: string
    roles: PlatformRole[]
    expiresAt: string
  }) => void
  clear: () => void
  _setHydrated: () => void
}

export const usePlatformStore = create<PlatformState>()(
  persist(
    (set) => ({
      token: null,
      platformUserId: null,
      email: null,
      displayName: null,
      roles: [],
      expiresAt: null,
      _hydrated: false,

      setSession(s) {
        set({
          token: s.token,
          platformUserId: s.platformUserId,
          email: s.email,
          displayName: s.displayName,
          roles: s.roles,
          expiresAt: s.expiresAt,
        })
      },

      clear() {
        set({
          token: null,
          platformUserId: null,
          email: null,
          displayName: null,
          roles: [],
          expiresAt: null,
        })
      },

      _setHydrated() {
        set({ _hydrated: true })
      },
    }),
    {
      name: "platform-auth",
      storage: createJSONStorage(() => sessionStorage),
      onRehydrateStorage: () => (state) => {
        state?._setHydrated()
      },
    }
  )
)
