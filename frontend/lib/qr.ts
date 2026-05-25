import { env } from "@/config/env"

export function buildQRUrl(qrToken: string, tenantSlug?: string | null): string {
  if (env.baseDomain && tenantSlug) {
    return `https://${tenantSlug}.${env.baseDomain}/table/${qrToken}`
  }
  const appOrigin = env.apiUrl.replace(":8080", ":3000")
  return `${appOrigin}/table/${qrToken}`
}
