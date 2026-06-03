"use client"

import { AnimatePresence, motion } from "framer-motion"
import { useWsStore } from "@/store/ws"

const prefersReducedMotion =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false

export function ReconnectingBanner() {
  const status = useWsStore((s) => s.status)
  const attempt = useWsStore((s) => s.attempt)
  const visible = status === "reconnecting" || status === "disconnected"

  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          key="banner"
          initial={prefersReducedMotion ? false : { y: -40, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={prefersReducedMotion ? undefined : { y: -40, opacity: 0 }}
          transition={{ duration: 0.2 }}
          role="status"
          aria-live="polite"
          className="w-full px-4 py-2 text-center text-xs font-medium"
          style={{
            backgroundColor: "var(--color-warning)",
            color: "var(--color-text)",
          }}
        >
          {status === "reconnecting"
            ? `Reconnecting${attempt > 1 ? ` (attempt ${attempt})` : ""}…`
            : "Connection lost. Please refresh."}
        </motion.div>
      )}
    </AnimatePresence>
  )
}
