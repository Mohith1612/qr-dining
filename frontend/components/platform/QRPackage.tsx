"use client"

import { useRef } from "react"
import dynamic from "next/dynamic"
import JSZip from "jszip"
import { buildQRUrl } from "@/lib/qr"
import { Button } from "@/components/ui/button"
import type { PlatformTable } from "@/types/platform"
import { Printer, Download } from "lucide-react"

// Canvas variant so we can export PNGs; rendered client-side only.
const QRCodeCanvas = dynamic(() => import("qrcode.react").then((m) => m.QRCodeCanvas), { ssr: false })

function slugify(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") || "branch"
}

// QRPackage renders the per-table QR codes and lets the operator print a sheet
// (browser print / save-as-PDF) or download a ZIP of per-table PNGs. The encoded
// URL is the guest table URL (buildQRUrl), identical to the staff QR flow.
export function QRPackage({ tables, slug, branchName }: { tables: PlatformTable[]; slug?: string | null; branchName: string }) {
  const gridRef = useRef<HTMLDivElement>(null)

  function collect(): { identifier: string; dataUrl: string }[] {
    const canvases = gridRef.current?.querySelectorAll("canvas") ?? []
    const out: { identifier: string; dataUrl: string }[] = []
    canvases.forEach((c, i) => {
      out.push({ identifier: tables[i]?.identifier ?? `T${i + 1}`, dataUrl: (c as HTMLCanvasElement).toDataURL("image/png") })
    })
    return out
  }

  async function downloadZip() {
    const items = collect()
    if (items.length === 0) return
    const zip = new JSZip()
    const folder = zip.folder(slugify(branchName)) ?? zip
    for (const { identifier, dataUrl } of items) {
      folder.file(`${identifier}.png`, dataUrl.split(",")[1], { base64: true })
    }
    const blob = await zip.generateAsync({ type: "blob" })
    const a = document.createElement("a")
    a.href = URL.createObjectURL(blob)
    a.download = `qr-${slugify(branchName)}.zip`
    a.click()
    URL.revokeObjectURL(a.href)
  }

  function printSheet() {
    const items = collect()
    if (items.length === 0) return
    const w = window.open("", "_blank")
    if (!w) return
    const cards = items
      .map((it) => `<div class="card"><img src="${it.dataUrl}" alt="${it.identifier}" /><div class="label">${it.identifier}</div></div>`)
      .join("")
    w.document.write(
      `<!doctype html><html><head><title>QR codes — ${branchName}</title><style>` +
        `body{font-family:system-ui,sans-serif;padding:28px;color:#1a1a1a}h1{font-size:18px;margin:0 0 18px}` +
        `.grid{display:flex;flex-wrap:wrap;gap:16px}.card{border:1px solid #ddd;border-radius:10px;padding:14px;text-align:center;width:190px;break-inside:avoid}` +
        `.card img{width:160px;height:160px}.label{margin-top:10px;font-weight:600;font-size:15px}` +
        `.noprint{margin-bottom:16px}@media print{.noprint{display:none}}` +
        `</style></head><body><h1>${branchName} — table QR codes</h1>` +
        `<button class="noprint" onclick="window.print()">Print / Save as PDF</button>` +
        `<div class="grid">${cards}</div>` +
        `<script>window.onload=function(){setTimeout(function(){window.print()},400)}</script></body></html>`
    )
    w.document.close()
  }

  return (
    <div>
      <div style={{ display: "flex", gap: 8, marginBottom: 14 }}>
        <Button size="sm" variant="outline" onClick={printSheet}><Printer size={14} /> Print sheet</Button>
        <Button size="sm" variant="outline" onClick={downloadZip}><Download size={14} /> Download ZIP</Button>
      </div>
      <div ref={gridRef} style={{ display: "flex", flexWrap: "wrap", gap: 12 }}>
        {tables.map((t) => (
          <div key={t.id} style={{ border: "1px solid var(--line-2)", borderRadius: "var(--rad-md)", padding: 10, textAlign: "center", background: "var(--bg-elev-1)" }}>
            <QRCodeCanvas value={buildQRUrl(t.qr_code_token, slug)} size={96} level="M" />
            <div className="mono" style={{ marginTop: 6, fontSize: 12, color: "var(--ink-2)" }}>{t.identifier}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
