"use client"

import { Divider, Eyebrow, Logo, QrChip, RendererProps, tableUrl, TableNumber, Wordmark } from "./shared"

// table_tent — folded double-sided tent. Front and back mirror the same content (logo,
// restaurant, QR, table, tagline) so the tent reads correctly from both sides of the table.
function TentFace({ config, branding, table }: RendererProps) {
  return (
    <div
      style={{
        flex: 1,
        background: "var(--bg-elev-1)",
        color: "var(--ink-1)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "2mm",
        padding: "8mm 6mm",
        textAlign: "center",
      }}
    >
      {config.showLogo && branding.logoUrl ? <Logo url={branding.logoUrl} maxH="10mm" /> : null}
      <Wordmark size="15pt">{branding.restaurantName}</Wordmark>
      {config.tagline ? <div style={{ fontSize: "8pt", color: "var(--ink-3)", fontStyle: "italic" }}>{config.tagline}</div> : null}
      <Divider width="45%" />
      <QrChip value={tableUrl(branding, table)} size={104} />
      <Eyebrow style={{ marginTop: "1mm" }}>Table</Eyebrow>
      <TableNumber size="26pt">{table.identifier}</TableNumber>
    </div>
  )
}

export function TableTent({ config, branding, table }: RendererProps) {
  return (
    <div
      style={{
        width: "100mm",
        height: "210mm",
        background: "var(--bg-base)",
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
    >
      <TentFace config={config} branding={branding} table={table} />
      {/* Fold line between the two mirrored faces. */}
      <div style={{ height: 0, borderTop: "1px dashed var(--line-3)" }} />
      <div style={{ flex: 1, transform: "rotate(180deg)", display: "flex" }}>
        <TentFace config={config} branding={branding} table={table} />
      </div>
    </div>
  )
}
