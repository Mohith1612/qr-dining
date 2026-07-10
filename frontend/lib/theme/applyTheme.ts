// Theme application for the live restaurant frontend. The structured theme
// (preset + custom CSS-var tokens) resolved from the backend is the authoritative
// source of tenant branding. It is applied directly to <html> (data-theme + inline
// CSS variables) and is NOT persisted to localStorage, so it never leaks across
// tenants/branches. next-themes provides the initial no-flash default; this overrides
// it once the tenant/branch theme is resolved.

export type ThemeConfig = {
  preset: string
  tokens: Record<string, string>
}

export const KNOWN_PRESETS = ["dark-luxury", "modern-minimal", "warm-cafe", "vibrant"] as const

// Mirrors the backend allowlist (services/theme.go allowedThemeTokens). Each key maps
// 1:1 to a `--<key>` CSS custom property defined in styles/themes.css.
const ALLOWED_THEME_TOKENS = new Set<string>([
  "accent", "accent-strong", "accent-soft", "accent-ink",
  "bg-base", "bg-elev-1", "bg-elev-2",
  "ink-1", "ink-2", "ink-3",
  "ok", "warn", "alert", "info",
])

const HEX_RE = /^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/

// Inline token vars we've applied, so a tenant/branch switch clears stale ones.
let appliedTokenKeys: string[] = []

export function validPreset(preset: string | undefined | null): boolean {
  return typeof preset === "string" && (KNOWN_PRESETS as readonly string[]).includes(preset)
}

// applyThemeTokens defensively applies only allowlisted keys with valid hex values.
// Unknown keys and non-hex values are ignored — no arbitrary CSS is ever applied.
export function applyThemeTokens(tokens: Record<string, string> | undefined | null) {
  if (typeof document === "undefined") return
  const root = document.documentElement
  for (const key of appliedTokenKeys) {
    root.style.removeProperty(`--${key}`)
  }
  const next: string[] = []
  if (tokens) {
    for (const [key, value] of Object.entries(tokens)) {
      if (!ALLOWED_THEME_TOKENS.has(key)) continue
      if (typeof value !== "string" || !HEX_RE.test(value)) continue
      root.style.setProperty(`--${key}`, value)
      next.push(key)
    }
  }
  appliedTokenKeys = next
}

// applyTheme sets the preset (data-theme) and custom tokens on <html>.
export function applyTheme(config: ThemeConfig | undefined | null) {
  if (typeof document === "undefined" || !config) return
  if (validPreset(config.preset)) {
    document.documentElement.dataset.theme = config.preset
  }
  applyThemeTokens(config.tokens)
}
