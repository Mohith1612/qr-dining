import JSZip from "jszip"
import type { CollateralBranding, CollateralConfig, CollateralTable } from "@/types/collateral"
import { buildPrintHtml, slugify } from "./print"
import { resolveColors } from "./theme"

// serializeSvg turns a rendered QRCodeSVG <svg> node into a standalone .svg document.
function serializeSvg(svg: SVGElement): string {
  const clone = svg.cloneNode(true) as SVGElement
  clone.setAttribute("xmlns", "http://www.w3.org/2000/svg")
  return `<?xml version="1.0" encoding="UTF-8"?>\n${new XMLSerializer().serializeToString(clone)}`
}

function triggerDownload(blob: Blob, filename: string) {
  const a = document.createElement("a")
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}

// downloadAssetPackage bundles a print-vendor-ready ZIP: per-table QR PNG + SVG, a
// self-contained themed print.html (Save-as-PDF), and the config. The hidden export grid
// (gridEl) must render one cell per table — in table order — each containing a <canvas>
// (QRCodeCanvas) and an <svg> (QRCodeSVG). themeNode is any node with the theme scope
// applied, used to resolve concrete colours for the standalone HTML.
export async function downloadAssetPackage(params: {
  gridEl: HTMLElement
  themeNode: HTMLElement
  tables: CollateralTable[]
  branding: CollateralBranding
  config: CollateralConfig
}): Promise<void> {
  const { gridEl, themeNode, tables, branding, config } = params
  const canvases = Array.from(gridEl.querySelectorAll("canvas")) as HTMLCanvasElement[]
  const svgs = Array.from(gridEl.querySelectorAll("svg")) as unknown as SVGElement[]
  if (canvases.length === 0) return

  const slug = slugify(branding.branchName || branding.restaurantName)
  const zip = new JSZip()
  const folder = zip.folder(slug) ?? zip
  const pngFolder = folder.folder("png") ?? folder
  const svgFolder = folder.folder("svg") ?? folder

  const qrPngByIdentifier: Record<string, string> = {}
  canvases.forEach((c, i) => {
    const id = tables[i]?.identifier ?? `T${i + 1}`
    const dataUrl = c.toDataURL("image/png")
    qrPngByIdentifier[id] = dataUrl
    pngFolder.file(`${id}.png`, dataUrl.split(",")[1], { base64: true })
  })
  svgs.forEach((s, i) => {
    const id = tables[i]?.identifier ?? `T${i + 1}`
    svgFolder.file(`${id}.svg`, serializeSvg(s))
  })

  const colors = resolveColors(themeNode)
  folder.file("print.html", buildPrintHtml(config, branding, tables, qrPngByIdentifier, colors))
  folder.file("config.json", JSON.stringify(config, null, 2))

  const blob = await zip.generateAsync({ type: "blob" })
  triggerDownload(blob, `collateral-${slug}.zip`)
}
