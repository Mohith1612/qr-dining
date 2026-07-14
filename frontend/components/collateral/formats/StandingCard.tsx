"use client"

import { branchLabel, Divider, Eyebrow, Logo, QrChip, RendererProps, tableUrl, TableNumber, Wordmark } from "./shared"

// standing_card — A6 portrait premium table card. The full hospitality piece:
// logo, restaurant, branch, welcome, subtitle, QR, table, WiFi, footer, socials.
export function StandingCard({ config, branding, table }: RendererProps) {
  const label = branchLabel(config, branding)
  const socials = [config.instagram ? `@${config.instagram.replace(/^@/, "")}` : "", config.website].filter(Boolean)
  return (
    <div
      style={{
        width: "105mm",
        height: "148mm",
        background: "var(--bg-base)",
        backgroundImage: "var(--glow-warm)",
        color: "var(--ink-1)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        padding: "9mm 8mm",
        position: "relative",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          width: "100%",
          height: "100%",
          background: "var(--bg-elev-1)",
          border: "1px solid var(--accent-soft)",
          borderRadius: 14,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: "2mm",
          padding: "7mm 6mm",
          textAlign: "center",
        }}
      >
        {config.showLogo && branding.logoUrl ? <Logo url={branding.logoUrl} /> : null}
        <Wordmark>{branding.restaurantName}</Wordmark>
        {config.showBranch && label ? (
          <div style={{ fontSize: "8pt", letterSpacing: "0.3em", textTransform: "uppercase", color: "var(--ink-3)" }}>{label}</div>
        ) : null}
        {config.tagline ? <div style={{ fontSize: "8.5pt", color: "var(--ink-3)", fontStyle: "italic" }}>{config.tagline}</div> : null}

        <Divider />

        {config.welcomeMessage ? (
          <div style={{ fontSize: "11pt", color: "var(--ink-2)", maxWidth: "82%", lineHeight: 1.3 }}>{config.welcomeMessage}</div>
        ) : null}

        <div style={{ margin: "1mm 0" }}>
          <QrChip value={tableUrl(branding, table)} size={118} />
        </div>

        <Eyebrow>Table</Eyebrow>
        <TableNumber>{table.identifier}</TableNumber>

        {config.subtitle ? <div style={{ fontSize: "8.5pt", color: "var(--ink-3)", maxWidth: "82%" }}>{config.subtitle}</div> : null}

        {config.showWifi && config.wifiName ? (
          <div style={{ fontSize: "8pt", color: "var(--ink-2)", marginTop: "1mm" }}>
            <span style={{ color: "var(--accent)", letterSpacing: "0.15em", textTransform: "uppercase", fontSize: "7pt", marginRight: 6 }}>WiFi</span>
            {config.wifiName}
            {config.wifiPassword ? <span style={{ color: "var(--ink-3)" }}> · {config.wifiPassword}</span> : null}
          </div>
        ) : null}

        {socials.length > 0 ? (
          <div style={{ fontSize: "7.5pt", color: "var(--ink-3)" }}>{socials.join("  ·  ")}</div>
        ) : null}

        {config.showFooter && config.footerNote ? (
          <>
            <Divider width="40%" />
            <div style={{ fontSize: "7.5pt", color: "var(--ink-3)" }}>{config.footerNote}</div>
          </>
        ) : null}
      </div>
    </div>
  )
}
