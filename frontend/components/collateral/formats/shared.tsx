"use client"

import dynamic from "next/dynamic"
import type { CSSProperties } from "react"
import { buildQRUrl } from "@/lib/qr"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"

const QRCodeSVG = dynamic(() => import("qrcode.react").then((m) => m.QRCodeSVG), { ssr: false })

// Props every format renderer receives. The theme is applied by an ancestor
// CollateralThemeScope, so renderers consume CSS variables (var(--accent), …) directly.
export interface RendererProps {
  config: CollateralConfig
  branding: CollateralBranding
  table: CollateralTable
}

export function tableUrl(branding: CollateralBranding, table: CollateralTable): string {
  return buildQRUrl(table.qr_code_token, branding.slug)
}

export function branchLabel(config: CollateralConfig, branding: CollateralBranding): string {
  return (config.branchDisplay || branding.branchName || "").trim()
}

// QrChip renders the QR on a clean white tile with dark modules. Kept consistent across
// themes for reliable scanning while the surrounding card stays fully theme-aware.
export function QrChip({ value, size, pad = 10 }: { value: string; size: number; pad?: number }) {
  return (
    <div style={{ background: "#FFFFFF", padding: pad, borderRadius: 8, lineHeight: 0, display: "inline-block" }}>
      <QRCodeSVG value={value} size={size} level="M" fgColor="#11100E" bgColor="#FFFFFF" />
    </div>
  )
}

export function Eyebrow({ children, style }: { children: React.ReactNode; style?: CSSProperties }) {
  return (
    <div style={{ fontSize: "7pt", letterSpacing: "0.4em", textTransform: "uppercase", color: "var(--accent)", fontWeight: 600, ...style }}>
      {children}
    </div>
  )
}

export function Divider({ width = "55%" }: { width?: string }) {
  return <div style={{ width, height: 1, background: "var(--line-3)", margin: "1.5mm 0" }} />
}

export function Wordmark({ children, size = "17pt" }: { children: React.ReactNode; size?: string }) {
  return (
    <div style={{ fontFamily: "var(--font-display)", fontSize: size, fontWeight: 600, letterSpacing: "0.02em", color: "var(--accent)", lineHeight: 1.05 }}>
      {children}
    </div>
  )
}

export function TableNumber({ children, size = "34pt" }: { children: React.ReactNode; size?: string }) {
  return (
    <div style={{ fontFamily: "var(--font-display)", fontSize: size, fontWeight: 500, letterSpacing: "-0.02em", lineHeight: 1, color: "var(--ink-1)" }}>
      {children}
    </div>
  )
}

export function Logo({ url, maxH = "12mm" }: { url: string; maxH?: string }) {
  // eslint-disable-next-line @next/next/no-img-element
  return <img src={url} alt="" style={{ maxWidth: "30mm", maxHeight: maxH, objectFit: "contain" }} />
}
