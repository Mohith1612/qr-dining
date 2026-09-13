import type { CollateralFormat } from "@/types/collateral"

// Content blocks a format may render. The config form and the renderers both read the
// per-format allow-set, so toggling a block the format doesn't support is a no-op
// (structured-only, no clutter).
export type CollateralBlock =
  | "logo"
  | "restaurant"
  | "branch"
  | "welcome"
  | "subtitle"
  | "qr"
  | "table"
  | "wifi"
  | "footer"
  | "tagline"
  | "socials"

export interface FormatMeta {
  key: CollateralFormat
  label: string
  description: string
  /** Physical print page size in millimetres (w × h). */
  page: { w: number; h: number }
  /** A4/operational sheet that packs many tables on one page. */
  multiPerPage: boolean
  /** Content blocks this format is allowed to render. */
  blocks: readonly CollateralBlock[]
}

// Format registry — mirrors backend services.allowedCollateralFormats and the spec's
// per-format content rules.
export const FORMAT_META: Record<CollateralFormat, FormatMeta> = {
  standing_card: {
    key: "standing_card",
    label: "Standing card",
    description: "A6 portrait premium table card — the full hospitality piece.",
    page: { w: 105, h: 148 },
    multiPerPage: false,
    blocks: ["logo", "restaurant", "branch", "welcome", "subtitle", "qr", "table", "wifi", "footer", "socials"],
  },
  table_tent: {
    key: "table_tent",
    label: "Table tent",
    description: "Folded double-sided tent — front and back share the table QR.",
    page: { w: 100, h: 210 },
    multiPerPage: false,
    blocks: ["logo", "restaurant", "qr", "table", "tagline"],
  },
  sticker: {
    key: "sticker",
    label: "Sticker",
    description: "Round, minimal — restaurant, QR and table only.",
    page: { w: 80, h: 80 },
    multiPerPage: false,
    blocks: ["restaurant", "qr", "table"],
  },
  square_card: {
    key: "square_card",
    label: "Square card",
    description: "Compact square — logo, QR and table.",
    page: { w: 100, h: 100 },
    multiPerPage: false,
    blocks: ["logo", "qr", "table"],
  },
  bulk_sheet: {
    key: "bulk_sheet",
    label: "Bulk sheet",
    description: "Operational A4 sheet — multiple tables per page, nothing decorative.",
    page: { w: 210, h: 297 },
    multiPerPage: true,
    blocks: ["restaurant", "table", "qr"],
  },
}

export const ALL_FORMATS = Object.values(FORMAT_META)

export function formatAllows(format: CollateralFormat, block: CollateralBlock): boolean {
  return FORMAT_META[format].blocks.includes(block)
}
