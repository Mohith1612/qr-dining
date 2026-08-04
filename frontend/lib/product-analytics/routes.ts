export type AppSurface = "guest" | "staff" | "platform" | "other"

const SESSION_ROUTE = /^\/session\/[^/]+(?:\/(menu|cart|orders|payment|assist))?\/?$/
const TABLE_ROUTE = /^\/table\/[^/]+\/?$/

export function routeTemplate(pathname: string): string {
  const clean = pathname.split(/[?#]/, 1)[0] || "/"
  if (TABLE_ROUTE.test(clean)) return "/table/:token"

  const sessionMatch = clean.match(SESSION_ROUTE)
  if (sessionMatch) {
    return sessionMatch[1] ? `/session/:id/${sessionMatch[1]}` : "/session/:id"
  }

  return clean
}

export function appSurface(pathname: string): AppSurface {
  const route = routeTemplate(pathname)
  if (route.startsWith("/table/") || route.startsWith("/session/")) return "guest"
  if (route === "/staff" || route.startsWith("/staff/")) return "staff"
  if (route === "/platform" || route.startsWith("/platform/")) return "platform"
  return "other"
}

export function sanitizeUrl(value: string): string {
  try {
    const parsed = new URL(value, typeof window === "undefined" ? "https://redacted.invalid" : window.location.origin)
    const route = routeTemplate(parsed.pathname)
    return parsed.origin === "https://redacted.invalid" ? route : `${parsed.origin}${route}`
  } catch {
    return routeTemplate(value)
  }
}
