import { env } from "@/config/env"
import { sanitizeUrl } from "@/lib/product-analytics/routes"
import type { CaptureResult, Properties } from "posthog-js"

const FORBIDDEN_KEY = /token|phone|password|pin|secret|fingerprint|authorization/i
const URL_KEY = /^\$(?:current_url|pathname|referrer)$/

function scrubValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(scrubValue)
  if (!value || typeof value !== "object" || value instanceof Date) return value
  return scrubProperties(value as Properties)
}

function scrubProperties(properties: Properties): Properties {
  const clean: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(properties)) {
    // PostHog's own project key is a required transport property. Preserve only
    // that exact known value; every application-level token field is dropped.
    if (FORBIDDEN_KEY.test(key) && !(key === "token" && value === env.posthogKey)) continue
    clean[key] = URL_KEY.test(key) && typeof value === "string" ? sanitizeUrl(value) : scrubValue(value)
  }
  return clean
}

export function scrubEvent(event: CaptureResult | null): CaptureResult | null {
  if (!event) return event
  return {
    ...event,
    properties: scrubProperties(event.properties),
    ...(event.$set ? { $set: scrubProperties(event.$set) } : {}),
    ...(event.$set_once ? { $set_once: scrubProperties(event.$set_once) } : {}),
  }
}
