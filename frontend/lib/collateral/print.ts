import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { FORMAT_META, formatAllows } from "./formats"
import type { ResolvedColors } from "./theme"

function esc(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c] as string))
}

export function slugify(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") || "branch"
}

// buildPrintHtml renders a self-contained, print-ready HTML sheet for print vendors. It
// uses concrete colours resolved from the active theme (no CSS variables) and embeds each
// table's QR as a PNG data URL. It mirrors the chosen format's content rules without being
// a pixel replica of the on-screen renderer — the in-app Print View is the high-fidelity path.
export function buildPrintHtml(
  config: CollateralConfig,
  branding: CollateralBranding,
  tables: CollateralTable[],
  qrPngByIdentifier: Record<string, string>,
  colors: ResolvedColors,
): string {
  const meta = FORMAT_META[config.format]
  const cardW = meta.page.w
  const cardH = meta.page.h
  const allows = (b: Parameters<typeof formatAllows>[1]) => formatAllows(config.format, b)
  const branchLabel = (config.branchDisplay || branding.branchName || "").trim()

  const card = (t: CollateralTable) => {
    const png = qrPngByIdentifier[t.identifier]
    const rows: string[] = []
    if (allows("logo") && config.showLogo && branding.logoUrl) {
      rows.push(`<img class="logo" src="${esc(branding.logoUrl)}" alt="" />`)
    }
    if (allows("restaurant")) rows.push(`<div class="brand">${esc(branding.restaurantName)}</div>`)
    if (allows("branch") && config.showBranch && branchLabel) rows.push(`<div class="branch">${esc(branchLabel)}</div>`)
    if (allows("welcome") && config.welcomeMessage) rows.push(`<div class="welcome">${esc(config.welcomeMessage)}</div>`)
    if (png) rows.push(`<div class="qr"><img src="${png}" alt="QR ${esc(t.identifier)}" /></div>`)
    if (allows("table")) rows.push(`<div class="tlabel">Table</div><div class="tnum">${esc(t.identifier)}</div>`)
    if (allows("subtitle") && config.subtitle) rows.push(`<div class="sub">${esc(config.subtitle)}</div>`)
    if (allows("tagline") && config.tagline) rows.push(`<div class="sub">${esc(config.tagline)}</div>`)
    if (allows("wifi") && config.showWifi && config.wifiName) {
      rows.push(`<div class="wifi"><span>WiFi</span> ${esc(config.wifiName)}${config.wifiPassword ? ` · ${esc(config.wifiPassword)}` : ""}</div>`)
    }
    if (allows("socials") && (config.instagram || config.website)) {
      const parts = [config.instagram ? `@${esc(config.instagram.replace(/^@/, ""))}` : "", config.website ? esc(config.website) : ""].filter(Boolean)
      rows.push(`<div class="social">${parts.join("&nbsp;·&nbsp;")}</div>`)
    }
    if (allows("footer") && config.showFooter && config.footerNote) rows.push(`<div class="foot">${esc(config.footerNote)}</div>`)
    return `<div class="card">${rows.join("")}</div>`
  }

  const cards = tables.map(card).join("")
  const pageSize = `${cardW}mm ${cardH}mm`

  return `<!doctype html><html><head><meta charset="utf-8" />
<title>QR collateral — ${esc(branding.restaurantName)}</title>
<style>
  * { box-sizing: border-box; }
  body { margin: 0; background: #f3f3f0; font-family: -apple-system, Segoe UI, system-ui, sans-serif; color: ${colors.ink1}; }
  .toolbar { padding: 16px; }
  .toolbar button { font: inherit; padding: 8px 14px; border-radius: 8px; border: 1px solid #ccc; background: #fff; cursor: pointer; }
  .sheet { display: flex; flex-wrap: wrap; gap: 10mm; padding: 10mm; justify-content: center; }
  .card {
    width: ${cardW}mm; min-height: ${cardH}mm; padding: 8mm 7mm; background: ${colors.bgElev1};
    color: ${colors.ink1}; border: 1px solid ${colors.line}; border-radius: 10px;
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3mm; text-align: center;
    break-inside: avoid;
  }
  .logo { max-width: 26mm; max-height: 14mm; object-fit: contain; }
  .brand { font-family: 'Cormorant Garamond', Georgia, serif; font-size: 16pt; font-weight: 600; color: ${colors.accent}; letter-spacing: 0.04em; }
  .branch { font-size: 8pt; letter-spacing: 0.28em; text-transform: uppercase; color: ${colors.ink3}; }
  .welcome { font-size: 10pt; color: ${colors.ink2}; max-width: 80%; }
  .qr { background: #fff; padding: 3mm; border-radius: 6px; line-height: 0; }
  .qr img { width: 28mm; height: 28mm; display: block; }
  .tlabel { font-size: 7pt; letter-spacing: 0.4em; text-transform: uppercase; color: ${colors.accent}; margin-top: 1mm; }
  .tnum { font-family: 'Cormorant Garamond', Georgia, serif; font-size: 30pt; line-height: 1; color: ${colors.ink1}; }
  .sub { font-size: 8.5pt; color: ${colors.ink3}; }
  .wifi { font-size: 8pt; color: ${colors.ink2}; }
  .wifi span { color: ${colors.accent}; letter-spacing: 0.15em; text-transform: uppercase; font-size: 7pt; }
  .social { font-size: 8pt; color: ${colors.ink3}; }
  .foot { font-size: 7.5pt; color: ${colors.ink3}; margin-top: 1mm; }
  @page { size: ${pageSize}; margin: 0; }
  @media print { .toolbar { display: none; } body { background: #fff; } .sheet { padding: 0; gap: 0; } }
</style></head>
<body>
  <div class="toolbar"><button onclick="window.print()">Print / Save as PDF</button></div>
  <div class="sheet">${cards}</div>
</body></html>`
}
