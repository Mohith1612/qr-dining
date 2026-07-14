"use client"

import { Eyebrow, Logo, QrChip, RendererProps, tableUrl, TableNumber } from "./shared"

// square_card — compact square. Logo, QR and table.
export function SquareCard({ config, branding, table }: RendererProps) {
  return (
    <div
      style={{
        width: "100mm",
        height: "100mm",
        background: "var(--bg-elev-1)",
        border: "1px solid var(--accent-soft)",
        borderRadius: 12,
        color: "var(--ink-1)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "2mm",
        padding: "8mm",
        textAlign: "center",
        overflow: "hidden",
      }}
    >
      {config.showLogo && branding.logoUrl ? <Logo url={branding.logoUrl} maxH="11mm" /> : null}
      <QrChip value={tableUrl(branding, table)} size={110} />
      <Eyebrow>Table</Eyebrow>
      <TableNumber size="24pt">{table.identifier}</TableNumber>
    </div>
  )
}
