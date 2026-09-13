import { create } from "zustand"

// Tenant branding (logo + display name) resolved per branch/session. Separate
// from the slug-based TenantProvider, which is empty when the app isn't served
// from a per-tenant subdomain (e.g. local multi-tenant testing). Both the guest
// and staff shells read from here so the header shows the tenant's logo and name.
interface BrandingState {
  logoUrl: string | null
  name: string | null
  setBranding: (branding: { logoUrl?: string | null; name?: string | null }) => void
  clear: () => void
}

export const useBrandingStore = create<BrandingState>((set) => ({
  logoUrl: null,
  name: null,
  setBranding: ({ logoUrl, name }) =>
    set({
      logoUrl: logoUrl && logoUrl.trim() ? logoUrl : null,
      name: name && name.trim() ? name : null,
    }),
  clear: () => set({ logoUrl: null, name: null }),
}))
