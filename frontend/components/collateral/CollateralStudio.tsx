"use client"

import { useMemo, useRef, useState } from "react"
import dynamic from "next/dynamic"
import { Printer, Download, Save, Loader2, ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { buildQRUrl } from "@/lib/qr"
import { FORMAT_META } from "@/lib/collateral/formats"
import { downloadAssetPackage } from "@/lib/collateral/export"
import type { ThemeConfig } from "@/lib/theme/applyTheme"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { CollateralThemeScope } from "./CollateralThemeScope"
import { FormatRenderer } from "./FormatRenderer"
import { CollateralConfigForm } from "./CollateralConfigForm"
import { CollateralPrintContainer } from "./CollateralPrintContainer"

const QRCodeCanvas = dynamic(() => import("qrcode.react").then((m) => m.QRCodeCanvas), { ssr: false })
const QRCodeSVG = dynamic(() => import("qrcode.react").then((m) => m.QRCodeSVG), { ssr: false })

// CollateralStudio is the surface-agnostic orchestrator: config form + live theme-aware
// preview + export bar. Mounted by both the platform operator page and the staff admin
// section. It is a *controlled* component — the parent owns the config state (and reloads
// it per branch), so the form never goes stale when the selected branch changes.
// Persistence is delegated to onSave (each surface uses its own trust domain).
export function CollateralStudio({
  branding,
  theme,
  tables,
  config,
  onConfigChange,
  onSave,
  canManage = true,
}: {
  branding: CollateralBranding
  theme: ThemeConfig | null
  tables: CollateralTable[]
  config: CollateralConfig
  onConfigChange: (config: CollateralConfig) => void
  onSave?: (config: CollateralConfig) => Promise<void>
  canManage?: boolean
}) {
  const [previewIdx, setPreviewIdx] = useState(0)
  const [saving, setSaving] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [printing, setPrinting] = useState(false)

  const exportGridRef = useRef<HTMLDivElement>(null)
  const themeNodeRef = useRef<HTMLDivElement>(null)

  const meta = FORMAT_META[config.format]
  const hasTables = tables.length > 0
  const previewTable = tables[Math.min(previewIdx, Math.max(0, tables.length - 1))]

  // Fit the physical card (mm) into the preview pane.
  const previewScale = useMemo(() => {
    const maxW = 360
    const mmToPx = 3.7795
    const w = meta.page.w * mmToPx
    return Math.min(1, maxW / w)
  }, [meta])

  async function handleSave() {
    if (!onSave) return
    setSaving(true)
    try {
      await onSave(config)
    } finally {
      setSaving(false)
    }
  }

  function handlePrint() {
    setPrinting(true)
    // Let the print container mount, then invoke the browser print dialog.
    setTimeout(() => {
      window.print()
      setTimeout(() => setPrinting(false), 300)
    }, 120)
  }

  async function handleExport() {
    if (!exportGridRef.current || !themeNodeRef.current) return
    setExporting(true)
    try {
      await downloadAssetPackage({
        gridEl: exportGridRef.current,
        themeNode: themeNodeRef.current,
        tables,
        branding,
        config,
      })
    } finally {
      setExporting(false)
    }
  }

  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 360px) 1fr", gap: 28, alignItems: "start" }}>
      {/* Left: configuration */}
      <div>
        <CollateralConfigForm config={config} onChange={onConfigChange} disabled={!canManage} />
      </div>

      {/* Right: preview + actions */}
      <div>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 14, flexWrap: "wrap" }}>
          <div style={{ display: "flex", gap: 8 }}>
            <Button size="sm" variant="outline" onClick={handlePrint} disabled={!hasTables}>
              <Printer size={14} /> Print view
            </Button>
            <Button size="sm" variant="outline" onClick={handleExport} disabled={!hasTables || exporting}>
              {exporting ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />} Download package
            </Button>
            {onSave && canManage ? (
              <Button size="sm" onClick={handleSave} disabled={saving}>
                {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />} Save
              </Button>
            ) : null}
          </div>
          {meta.multiPerPage ? null : (
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <Button size="icon-sm" variant="outline" onClick={() => setPreviewIdx((i) => Math.max(0, i - 1))} disabled={previewIdx <= 0}>
                <ChevronLeft size={14} />
              </Button>
              <span style={{ fontSize: 12, color: "var(--ink-3)", minWidth: 84, textAlign: "center" }}>
                {hasTables ? `Table ${previewTable?.identifier}` : "No tables"}
              </span>
              <Button size="icon-sm" variant="outline" onClick={() => setPreviewIdx((i) => Math.min(tables.length - 1, i + 1))} disabled={previewIdx >= tables.length - 1}>
                <ChevronRight size={14} />
              </Button>
            </div>
          )}
        </div>

        {/* Live theme-aware preview */}
        <div style={{ background: "var(--bg-elev-2)", border: "1px solid var(--line-2)", borderRadius: "var(--rad-lg)", padding: 24, display: "flex", justifyContent: "center", overflow: "auto" }}>
          {hasTables && previewTable ? (
            <div style={{ transform: `scale(${previewScale})`, transformOrigin: "top center" }}>
              <CollateralThemeScope ref={themeNodeRef} theme={theme} style={{ boxShadow: "var(--shadow-3)", borderRadius: meta.key === "sticker" ? "50%" : 12, overflow: "hidden" }}>
                <FormatRenderer config={config} branding={branding} table={previewTable} tables={tables} />
              </CollateralThemeScope>
            </div>
          ) : (
            <p style={{ color: "var(--ink-3)", fontSize: 14, padding: "40px 0" }}>This branch has no tables yet. Create tables first to generate collateral.</p>
          )}
        </div>
        <p style={{ fontSize: 12, color: "var(--ink-4)", marginTop: 10 }}>
          {meta.label} · {meta.page.w}×{meta.page.h}mm · theme “{theme?.preset ?? "dark-luxury"}”. Print view paginates one per page
          {meta.multiPerPage ? " (bulk sheet packs many per page)" : ""}; download bundles QR PNG + SVG and a print-ready HTML sheet.
        </p>
      </div>

      {/* Hidden export grid: one QR canvas + svg per table, in table order. */}
      <div ref={exportGridRef} aria-hidden style={{ position: "absolute", left: -99999, top: 0, width: 1, height: 1, overflow: "hidden" }}>
        {tables.map((t) => (
          <div key={t.id}>
            <QRCodeCanvas value={buildQRUrl(t.qr_code_token, branding.slug)} size={420} level="M" fgColor="#11100E" bgColor="#FFFFFF" />
            <QRCodeSVG value={buildQRUrl(t.qr_code_token, branding.slug)} size={420} level="M" fgColor="#11100E" bgColor="#FFFFFF" />
          </div>
        ))}
      </div>

      {/* Print container (rendered only while printing). */}
      {printing && hasTables ? (
        <CollateralPrintContainer config={config} branding={branding} theme={theme} tables={tables} />
      ) : null}
    </div>
  )
}
