import { NextRequest, NextResponse } from "next/server"

// Extracts the restaurant tenant slug from the request hostname.
// "olive.dining.example.com" with baseDomain "dining.example.com" → "olive"
// Returns null when running on localhost or when the host doesn't match baseDomain.
function extractTenantSlug(host: string, baseDomain: string | null): string | null {
  if (!baseDomain) return null

  // Strip port.
  const hostname = host.split(":")[0]
  const suffix = `.${baseDomain}`

  if (!hostname.endsWith(suffix)) return null

  const slug = hostname.slice(0, -suffix.length)
  // Reject nested subdomains.
  if (slug.includes(".") || !slug) return null

  return slug
}

export function middleware(request: NextRequest) {
  const host = request.headers.get("host") ?? ""
  const baseDomain = process.env.NEXT_PUBLIC_BASE_DOMAIN ?? null
  const slug = extractTenantSlug(host, baseDomain)

  const response = NextResponse.next()

  // Pass the resolved tenant slug downstream via a response header.
  // The TenantProvider reads this on the client to avoid re-parsing the hostname.
  if (slug) {
    response.headers.set("x-tenant-slug", slug)
  }

  return response
}

export const config = {
  // Run on all paths except static files and Next.js internals.
  matcher: ["/((?!_next/static|_next/image|favicon.ico|manifest.json).*)"],
}
