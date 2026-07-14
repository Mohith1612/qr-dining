"use client"

import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { StandingCard } from "./formats/StandingCard"
import { TableTent } from "./formats/TableTent"
import { Sticker } from "./formats/Sticker"
import { SquareCard } from "./formats/SquareCard"
import { BulkSheet } from "./formats/BulkSheet"

// FormatRenderer dispatches to the chosen format renderer. Single-table formats render the
// given table; bulk_sheet renders the whole list. Must be rendered inside a
// CollateralThemeScope so theme CSS variables resolve.
export function FormatRenderer({
  config,
  branding,
  table,
  tables,
}: {
  config: CollateralConfig
  branding: CollateralBranding
  table: CollateralTable
  tables: CollateralTable[]
}) {
  switch (config.format) {
    case "table_tent":
      return <TableTent config={config} branding={branding} table={table} />
    case "sticker":
      return <Sticker config={config} branding={branding} table={table} />
    case "square_card":
      return <SquareCard config={config} branding={branding} table={table} />
    case "bulk_sheet":
      return <BulkSheet config={config} branding={branding} tables={tables} />
    case "standing_card":
    default:
      return <StandingCard config={config} branding={branding} table={table} />
  }
}
