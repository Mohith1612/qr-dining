"use client"

import { buildQRUrl } from "@/lib/qr"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { QrChip } from "./shared"

// bulk_sheet — operational A4 sheet, multiple tables per page, nothing decorative.
// Renders all tables in a grid (restaurant + table + QR only).
export function BulkSheet({
  config,
  branding,
  tables,
}: {
  config: CollateralConfig
  branding: CollateralBranding
  tables: CollateralTable[]
}) {
  return (
    <div
      style={{
        width: "210mm",
        minHeight: "297mm",
        background: "var(--bg-base)",
        color: "var(--ink-1)",
        padding: "12mm",
        boxSizing: "border-box",
      }}
    >
      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", marginBottom: "8mm", borderBottom: "1px solid var(--line-3)", paddingBottom: "3mm" }}>
        <div style={{ fontFamily: "var(--font-display)", fontSize: "18pt", fontWeight: 600, color: "var(--accent)" }}>{branding.restaurantName}</div>
        <div style={{ fontSize: "8pt", letterSpacing: "0.2em", textTransform: "uppercase", color: "var(--ink-3)" }}>
          {(config.branchDisplay || branding.branchName || "Tables")} · {tables.length} tables
        </div>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: "6mm" }}>
        {tables.map((t) => (
          <div
            key={t.id}
            style={{
              background: "var(--bg-elev-1)",
              border: "1px solid var(--line-2)",
              borderRadius: 8,
              padding: "5mm",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: "2mm",
              breakInside: "avoid",
            }}
          >
            <QrChip value={buildQRUrl(t.qr_code_token, branding.slug)} size={84} pad={6} />
            <div style={{ fontSize: "7pt", letterSpacing: "0.3em", textTransform: "uppercase", color: "var(--ink-3)" }}>Table</div>
            <div style={{ fontFamily: "var(--font-display)", fontSize: "16pt", color: "var(--ink-1)", lineHeight: 1 }}>{t.identifier}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
