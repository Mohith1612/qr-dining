"use client"

import { useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { AnimatePresence, motion } from "framer-motion"
import { Timer } from "lucide-react"
import { toast } from "sonner"
import { useSessionStore } from "@/store/session"
import { assistanceApi } from "@/lib/api/assistance"
import { sessionsApi } from "@/lib/api/sessions"

const prefersReducedMotion =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false

export function SessionTimeoutBanner() {
  const router = useRouter()
  const sessionExpiringAt = useSessionStore((s) => s.sessionExpiringAt)
  const completedPayment = useSessionStore((s) => s.completedPayment)
  const session = useSessionStore((s) => s.session)

  const [secondsLeft, setSecondsLeft] = useState(0)
  const lastAnnouncedMinute = useRef(-1)

  const visible = !!sessionExpiringAt && !completedPayment

  // Sync secondsLeft with expiresAt, ticking every second
  useEffect(() => {
    if (!sessionExpiringAt) return
    const update = () => {
      setSecondsLeft(Math.max(0, Math.round((sessionExpiringAt.getTime() - Date.now()) / 1000)))
    }
    update()
    const id = setInterval(update, 1000)
    return () => clearInterval(id)
  }, [sessionExpiringAt])

  // Escape key dismisses
  useEffect(() => {
    if (!visible) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") useSessionStore.getState().setSessionExpiringAt(null)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [visible])

  const isUrgent = secondsLeft < 5 * 60
  const displayMinutes = Math.floor(secondsLeft / 60)
  const displaySeconds = String(secondsLeft % 60).padStart(2, "0")

  // Announce remaining time to screen readers once per minute
  const ariaMinutes = Math.floor(secondsLeft / 60)
  const ariaLabel =
    ariaMinutes !== lastAnnouncedMinute.current ? `${ariaMinutes} minutes remaining` : undefined
  if (ariaLabel) lastAnnouncedMinute.current = ariaMinutes

  // "We're still ordering" — explicitly reactivate the session so the next
  // order doesn't 409. Applies the returned (active) session to local state.
  async function handleStillOrdering() {
    useSessionStore.getState().setSessionExpiringAt(null)
    if (!session) return
    try {
      const guestToken = sessionStorage.getItem("guest_access_token") ?? ""
      const active = await sessionsApi.reactivate(session.id, guestToken)
      useSessionStore.getState().applyReactivated(active)
      toast.success("You're all set — take your time.")
    } catch {
      toast.error("Couldn't keep the table open. Please refresh and try again.")
    }
  }

  // Urgent phase — guest asks a waiter to keep the table.
  async function handleExtend() {
    useSessionStore.getState().setSessionExpiringAt(null)
    if (session) {
      try {
        const guestToken = sessionStorage.getItem("guest_access_token") ?? ""
        await assistanceApi.request(session.id, session.table_id, guestToken, "waiter")
      } catch {}
    }
    toast.success("We've let your waiter know you're staying a while.")
  }

  function handleBill() {
    if (session) router.push(`/session/${session.id}/payment`)
  }

  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          key="timeout-banner"
          role="status"
          aria-live="polite"
          aria-label={ariaLabel}
          initial={prefersReducedMotion ? false : { y: 60, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={prefersReducedMotion ? undefined : { y: 60, opacity: 0 }}
          transition={{ duration: 0.25, ease: [0.22, 0.61, 0.36, 1] }}
          style={{
            position: "fixed",
            bottom: "calc(84px + env(safe-area-inset-bottom))",
            left: 0,
            right: 0,
            zIndex: 45,
            background: "var(--bg-elev-2)",
            borderTop: "1px solid var(--line-2)",
            padding: "16px 20px",
          }}
        >
          {isUrgent ? (
            // Urgent phase — countdown + stronger CTAs
            <div>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  marginBottom: 4,
                }}
              >
                <Timer size={16} style={{ color: "var(--alert)", flexShrink: 0 }} aria-hidden />
                <span
                  style={{
                    fontSize: 18,
                    fontWeight: 700,
                    color: "var(--alert)",
                    fontVariantNumeric: "tabular-nums",
                  }}
                >
                  {displayMinutes}:{displaySeconds}
                </span>
                <span style={{ fontSize: 13, color: "var(--ink-2)", fontWeight: 500 }}>
                  remaining
                </span>
              </div>
              <p style={{ fontSize: 13, color: "var(--ink-3)", marginBottom: 12 }}>
                Your table will close soon. Need more time? Request a waiter.
              </p>
              <BannerButtons
                primary={{ label: "Request waiter", onClick: handleExtend }}
                secondary={{ label: "Pay now", onClick: handleBill }}
              />
            </div>
          ) : (
            // Warning phase — calm message, two options
            <div>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  marginBottom: 4,
                }}
              >
                <Timer size={16} style={{ color: "var(--warn)", flexShrink: 0 }} aria-hidden />
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--ink-1)" }}>
                  Still here?
                </span>
              </div>
              <p style={{ fontSize: 13, color: "var(--ink-3)", marginBottom: 12 }}>
                Your table will close in about {displayMinutes} minutes.
              </p>
              <BannerButtons
                primary={{ label: "We’re still ordering", onClick: handleStillOrdering }}
                secondary={{ label: "Request the bill", onClick: handleBill }}
              />
            </div>
          )}
        </motion.div>
      )}
    </AnimatePresence>
  )
}

interface BtnDef {
  label: string
  onClick: () => void
}

function BannerButtons({ primary, secondary }: { primary: BtnDef; secondary: BtnDef }) {
  return (
    <div
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: 8,
      }}
    >
      <button
        onClick={primary.onClick}
        style={{
          flex: "1 1 auto",
          minWidth: 120,
          padding: "9px 16px",
          borderRadius: 999,
          background: "var(--accent)",
          color: "var(--accent-ink)",
          fontSize: 13,
          fontWeight: 600,
          cursor: "pointer",
          border: "none",
          whiteSpace: "nowrap",
        }}
      >
        {primary.label}
      </button>
      <button
        onClick={secondary.onClick}
        style={{
          flex: "1 1 auto",
          minWidth: 120,
          padding: "9px 16px",
          borderRadius: 999,
          background: "transparent",
          color: "var(--ink-2)",
          fontSize: 13,
          fontWeight: 500,
          cursor: "pointer",
          border: "1px solid var(--line-3)",
          whiteSpace: "nowrap",
        }}
      >
        {secondary.label}
      </button>
    </div>
  )
}
