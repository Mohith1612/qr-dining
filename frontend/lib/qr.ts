import { env } from "@/config/env"

// Resolve the public guest origin a QR code should encode. This must be the
// FRONTEND origin (where /table/:token is served), never the API origin.
function guestOrigin(): string {
  if (env.guestUrl) return env.guestUrl.replace(/\/$/, "")
  if (typeof window !== "undefined" && window.location?.origin) return window.location.origin
  return "http://localhost:3000"
}

export function buildQRUrl(qrToken: string, tenantSlug?: string | null): string {
  if (env.baseDomain && tenantSlug) {
    return `https://${tenantSlug}.${env.baseDomain}/table/${qrToken}`
  }
  return `${guestOrigin()}/table/${qrToken}`
}
