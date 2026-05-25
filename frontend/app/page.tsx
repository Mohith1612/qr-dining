import { QrCode } from "lucide-react"

export default function Home() {
  return (
    <div
      className="atmos min-h-svh flex flex-col items-center justify-center screen-enter"
      style={{ background: "var(--bg-base)", position: "relative" }}
    >
      {/* Atmospheric warmth gradient */}
      <div style={{ position: "absolute", inset: 0, background: "var(--glow-warm)", pointerEvents: "none" }} />

      <div className="relative z-10 flex flex-col items-center px-8 text-center" style={{ maxWidth: 340 }}>
        {/* Brand mark */}
        <div style={{ marginBottom: 36 }}>
          <div style={{
            width: 88, height: 88, borderRadius: 26,
            background: "linear-gradient(160deg, var(--bg-elev-2), var(--bg-elev-1))",
            border: "1px solid var(--line-2)",
            boxShadow: "var(--shadow-3)",
            display: "flex", alignItems: "center", justifyContent: "center",
            position: "relative", margin: "0 auto",
          }}>
            <QrCode size={40} style={{ color: "var(--accent)" }} aria-hidden />
            <span style={{
              position: "absolute", inset: -1, borderRadius: 27,
              background: "linear-gradient(180deg, var(--accent-soft), transparent 50%)",
              pointerEvents: "none", opacity: 0.6,
            }} />
          </div>
        </div>

        <p className="eyebrow" style={{ marginBottom: 14 }}>Hospitality, in your hand</p>
        <h1 className="serif" style={{
          margin: 0, fontSize: 44, fontWeight: 500, letterSpacing: "-0.02em",
          color: "var(--ink-1)", lineHeight: 1.05, marginBottom: 16,
        }}>
          QR Dining
        </h1>
        <p style={{ color: "var(--ink-2)", fontSize: 14, lineHeight: 1.55, maxWidth: 280, margin: "0 auto" }}>
          Scan the small folded card on your table to begin.
          Your party can order, request, and pay together.
        </p>

        <div style={{
          marginTop: 48, display: "flex", justifyContent: "center", gap: 6, opacity: 0.6,
          color: "var(--ink-3)", fontSize: 11, letterSpacing: "0.16em", textTransform: "uppercase",
        }}>
          <span>Realtime</span><span>·</span><span>Collaborative</span><span>·</span><span>Cardless</span>
        </div>
      </div>
    </div>
  )
}
