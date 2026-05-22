export const env = {
  apiUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  wsUrl: process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080",
  isDev: process.env.NEXT_PUBLIC_ENV === "development",
  // Multi-tenant: set NEXT_PUBLIC_TENANT_SLUG for local dev to scope the app to one restaurant.
  // In production, the tenant slug is extracted from the subdomain automatically.
  tenantSlug: process.env.NEXT_PUBLIC_TENANT_SLUG ?? null,
  baseDomain: process.env.NEXT_PUBLIC_BASE_DOMAIN ?? null,
} as const
