"use client"

import { useEffect, useState } from "react"
import { WifiOff } from "lucide-react"

const REACTIVATION_WINDOW_MS = 5 * 60 * 1000

export function SessionReactivatingBanner() {
  const [secondsLeft, setSecondsLeft] = useState(REACTIVATION_WINDOW_MS / 1000)

  useEffect(() => {
    const interval = setInterval(() => {
      setSecondsLeft((s) => Math.max(0, s - 1))
    }, 1000)
    return () => clearInterval(interval)
  }, [])

  const minutes = Math.floor(secondsLeft / 60)
  const seconds = secondsLeft % 60
  const countdown = `${minutes}:${String(seconds).padStart(2, "0")}`

  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        background: "var(--warn, #d97706)",
        color: "#fff",
        padding: "10px 16px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 12,
        fontSize: 13,
        fontWeight: 500,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8, flex: 1 }}>
        <WifiOff size={16} aria-hidden />
        <span>Your table is paused. Reconnecting… ({countdown} left)</span>
      </div>
      <button
        onClick={() => window.location.reload()}
        style={{
          background: "rgba(255,255,255,0.2)",
          border: "1px solid rgba(255,255,255,0.4)",
          color: "#fff",
          borderRadius: 6,
          padding: "4px 10px",
          fontSize: 12,
          fontWeight: 600,
          cursor: "pointer",
          whiteSpace: "nowrap",
        }}
      >
        Refresh now
      </button>
    </div>
  )
}
