"use client"

import { Eyebrow, QrChip, RendererProps, tableUrl, TableNumber, Wordmark } from "./shared"

// sticker — round, minimal. Restaurant, QR and table only; nothing else.
export function Sticker({ branding, table }: RendererProps) {
  return (
    <div
      style={{
        width: "80mm",
        height: "80mm",
        borderRadius: "50%",
        background: "var(--bg-elev-1)",
        backgroundImage: "var(--glow-warm)",
        border: "1px solid var(--accent-soft)",
        color: "var(--ink-1)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "1.5mm",
        padding: "8mm",
        textAlign: "center",
        overflow: "hidden",
      }}
    >
      <Wordmark size="12pt">{branding.restaurantName}</Wordmark>
      <QrChip value={tableUrl(branding, table)} size={88} pad={8} />
      <Eyebrow style={{ fontSize: "6pt" }}>Table</Eyebrow>
      <TableNumber size="18pt">{table.identifier}</TableNumber>
    </div>
  )
}
