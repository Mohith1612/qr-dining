import type { CSSProperties } from "react"

// Scoped theme application for collateral. The live guest UI applies the theme to <html>
// (lib/theme/applyTheme.ts); collateral instead applies it to a wrapper <div> so a preview
// can show a theme different from the operator's own console theme. Because styles/themes.css
// targets [data-theme="X"] with an *attribute* selector (not just :root), all preset tokens
// cascade to that subtree; custom tokens are layered as inline CSS variables here.

// Mirrors the backend allowlist (services/theme.go) and lib/theme/applyTheme.ts.
const ALLOWED_THEME_TOKENS = new Set<string>([
  "accent", "accent-strong", "accent-soft", "accent-ink",
  "bg-base", "bg-elev-1", "bg-elev-2",
  "ink-1", "ink-2", "ink-3",
  "ok", "warn", "alert", "info",
])

const HEX_RE = /^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/

// themeScopeStyle returns the inline custom-token CSS variables to layer over a preset on
// the scope wrapper. Only allowlisted keys with valid hex values are applied (no arbitrary CSS).
export function themeScopeStyle(tokens: Record<string, string> | undefined | null): CSSProperties {
  const style: Record<string, string> = {}
  if (tokens) {
    for (const [key, value] of Object.entries(tokens)) {
      if (!ALLOWED_THEME_TOKENS.has(key)) continue
      if (typeof value !== "string" || !HEX_RE.test(value)) continue
      style[`--${key}`] = value
    }
  }
  return style as CSSProperties
}

export interface ResolvedColors {
  bgBase: string
  bgElev1: string
  bgElev2: string
  ink1: string
  ink2: string
  ink3: string
  accent: string
  accentStrong: string
  accentInk: string
  line: string
}

// resolveColors reads the concrete (computed) token values from a node that has the theme
// scope applied, so an exported standalone print.html can embed real colours rather than
// CSS variables a print vendor's browser wouldn't resolve.
export function resolveColors(node: HTMLElement): ResolvedColors {
  const s = getComputedStyle(node)
  const v = (name: string, fallback: string) => {
    const got = s.getPropertyValue(name).trim()
    return got || fallback
  }
  return {
    bgBase: v("--bg-base", "#0E0C09"),
    bgElev1: v("--bg-elev-1", "#181410"),
    bgElev2: v("--bg-elev-2", "#211C16"),
    ink1: v("--ink-1", "#F4E8D1"),
    ink2: v("--ink-2", "#C8B894"),
    ink3: v("--ink-3", "#8A7E63"),
    accent: v("--accent", "#C9A876"),
    accentStrong: v("--accent-strong", "#D8B98A"),
    accentInk: v("--accent-ink", "#1A1410"),
    line: v("--line-3", "rgba(244,232,209,0.18)"),
  }
}
