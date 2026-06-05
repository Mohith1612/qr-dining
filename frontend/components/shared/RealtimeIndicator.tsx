"use client"

import { useWsStore } from "@/store/ws"

export function RealtimeIndicator() {
  const status = useWsStore((s) => s.status)

  const map = {
    connected: { color: "var(--color-success)", label: "Connected" },
    reconnecting: { color: "var(--color-warning)", label: "Reconnecting" },
    disconnected: { color: "var(--color-text-muted)", label: "Disconnected" },
    failed: { color: "var(--color-text-muted)", label: "Disconnected" },
  }

  const { color, label } = map[status]

  return (
    <span
      role="status"
      aria-label={label}
      className="inline-block size-2 rounded-full flex-shrink-0"
      style={{ backgroundColor: color }}
    />
  )
}
