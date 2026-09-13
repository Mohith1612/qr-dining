"use client"

import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { FORMAT_META } from "@/lib/collateral/formats"
import { CollateralThemeScope } from "./CollateralThemeScope"
import { FormatRenderer } from "./FormatRenderer"

// CollateralPrintContainer renders the high-fidelity, theme-aware print pages in-document
// and hides everything else under @media print (same technique as admin/PrintTemplate.tsx,
// so styles/themes.css and the display fonts are already loaded and theme vars resolve).
// Single-table formats emit one page per table; bulk_sheet emits a single packed sheet.
export function CollateralPrintContainer({
  config,
  branding,
  theme,
  tables,
}: {
  config: CollateralConfig
  branding: CollateralBranding
  theme: ThemeConfig | null
  tables: CollateralTable[]
}) {
  const meta = FORMAT_META[config.format]
  const pages: CollateralTable[] = meta.multiPerPage ? [tables[0]] : tables

  return (
    <div id="collateral-print-root">
      <style>{`
        @media print {
          body > *:not(#collateral-print-root) { display: none !important; }
          #collateral-print-root { display: block !important; }
          /* Preserve themed backgrounds + colours in print/PDF (else cards collapse to plain text). */
          #collateral-print-root, #collateral-print-root * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
          .collateral-print-page { break-after: page; page-break-after: always; }
          .collateral-print-page:last-child { break-after: auto; page-break-after: auto; }
        }
        @media screen { #collateral-print-root { display: none; } }
        @page { size: ${meta.page.w}mm ${meta.page.h}mm; margin: 0; }
      `}</style>
      {pages.map((t, i) => (
        <div key={t?.id ?? i} className="collateral-print-page" style={{ display: "flex", justifyContent: "center" }}>
          <CollateralThemeScope theme={theme}>
            <FormatRenderer config={config} branding={branding} table={t} tables={tables} />
          </CollateralThemeScope>
        </div>
      ))}
    </div>
  )
}
