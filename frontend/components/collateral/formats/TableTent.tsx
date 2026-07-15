"use client"

import { Divider, Eyebrow, Logo, QrChip, RendererProps, tableUrl, TableNumber, Wordmark } from "./shared"

// table_tent — folded double-sided tent. The card folds with the centre crease as the top
// ridge and both panels hang down to the table. So each face's head (logo/brand) must sit at
// the fold and content flow outward toward the free edge: the TOP panel is rotated 180° so
// that, once folded, both sides read upright (brand at the ridge, table number at the base).
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
      {/* Top panel rotated 180° so its head meets the ridge — reads upright once folded. */}
      <div style={{ flex: 1, transform: "rotate(180deg)", display: "flex" }}>
        <TentFace config={config} branding={branding} table={table} />
      </div>
      {/* Centre crease = the tent ridge. Both heads meet here so the text reads outward. */}
      <div style={{ height: 0, borderTop: "1px dashed var(--line-3)" }} />
      <TentFace config={config} branding={branding} table={table} />
    </div>
  )
}
