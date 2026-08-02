"use client"

import { AnimatePresence, motion } from "framer-motion"
import { useWsStore } from "@/store/ws"

const prefersReducedMotion =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false

interface ReconnectingBannerProps {
  onRetry: () => void
}

export function ReconnectingBanner({ onRetry }: ReconnectingBannerProps) {
  const status = useWsStore((s) => s.status)
  const attempt = useWsStore((s) => s.attempt)
  const visible =
    status === "reconnecting" || status === "disconnected" || status === "failed"

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
            backgroundColor: "var(--warn-soft)",
            color: "var(--warn)",
            borderBottom: "1px solid var(--warn)",
          }}
        >
          {status === "reconnecting" ? (
            `Reconnecting${attempt > 1 ? ` (attempt ${attempt})` : ""}…`
          ) : status === "failed" ? (
            <span className="inline-flex items-center gap-2">
              <span>Connection lost.</span>
              <button
                type="button"
                onClick={onRetry}
                className="rounded-full border border-current px-3 py-1 font-semibold"
              >
                Retry
              </button>
            </span>
          ) : (
            "Connection lost. Reconnecting…"
          )}
        </motion.div>
      )}
    </AnimatePresence>
  )
}
