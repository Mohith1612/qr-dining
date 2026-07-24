export const env = {
  apiUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  wsUrl: process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080",
  isDev: process.env.NEXT_PUBLIC_ENV === "development",
  // Multi-tenant: set NEXT_PUBLIC_TENANT_SLUG for local dev to scope the app to one restaurant.
  // In production, the tenant slug is extracted from the subdomain automatically.
  tenantSlug: process.env.NEXT_PUBLIC_TENANT_SLUG ?? null,
  baseDomain: process.env.NEXT_PUBLIC_BASE_DOMAIN ?? null,
  // The public guest origin QR codes point at (the FRONTEND, not the API).
  // Falls back to the current browser origin so local/dev QR codes resolve to
  // whatever port the frontend is actually served on.
  guestUrl: process.env.NEXT_PUBLIC_GUEST_URL ?? null,
} as const
