import { QrCode } from "lucide-react"

export default function Home() {
  return (
    <div
      className="min-h-svh flex flex-col items-center justify-center px-6 text-center gap-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div
        className="size-16 rounded-2xl flex items-center justify-center"
        style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
      >
        <QrCode className="size-8" style={{ color: "var(--color-accent)" }} aria-hidden />
      </div>
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">QR Dining</h1>
        <p className="text-sm max-w-xs" style={{ color: "var(--color-text-muted)" }}>
          Scan the QR code at your table to begin ordering
        </p>
      </div>
    </div>
  )
}
