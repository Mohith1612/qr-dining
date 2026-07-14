// Premium QR collateral — shared types for both the platform operator studio and the
// staff admin section. This is the *physical/content* concern only (chosen print format,
// content toggles, text/WiFi/social content). The theme stays the source of truth for
// colour/typography; the renderer composes Theme + Branch metadata + this config.

export const COLLATERAL_FORMATS = [
  "standing_card",
  "table_tent",
  "sticker",
  "square_card",
  "bulk_sheet",
] as const

export type CollateralFormat = (typeof COLLATERAL_FORMATS)[number]

export interface CollateralConfig {
  format: CollateralFormat
  showLogo: boolean
  showBranch: boolean
  showWifi: boolean
  showFooter: boolean
  welcomeMessage: string
  subtitle: string
  footerNote: string
  branchDisplay: string
  tagline: string
  wifiName: string
  wifiPassword: string
  instagram: string
  website: string
}

// Mirrors backend services.DefaultCollateralConfig.
export const DEFAULT_COLLATERAL: CollateralConfig = {
  format: "standing_card",
  showLogo: true,
  showBranch: true,
  showWifi: false,
  showFooter: true,
  welcomeMessage: "",
  subtitle: "",
  footerNote: "",
  branchDisplay: "",
  tagline: "",
  wifiName: "",
  wifiPassword: "",
  instagram: "",
  website: "",
}

// Branding inputs resolved from the restaurant/branch records (not the theme).
export interface CollateralBranding {
  restaurantName: string
  branchName: string
  logoUrl?: string
  /** tenant slug used to build the guest table URL (buildQRUrl). */
  slug?: string | null
}

export interface CollateralTable {
  id: number
  identifier: string
  qr_code_token: string
}

// Merge a partial config (e.g. from the API) over defaults so the UI always has every
// field present, and coerce an unknown persisted format back to the default.
export function normalizeCollateralConfig(input: Partial<CollateralConfig> | null | undefined): CollateralConfig {
  const merged = { ...DEFAULT_COLLATERAL, ...(input ?? {}) }
  if (!COLLATERAL_FORMATS.includes(merged.format)) merged.format = DEFAULT_COLLATERAL.format
  return merged
}
