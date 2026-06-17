"use client"

import { ReactNode, useEffect, useRef } from "react"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  className?: string
}

const FOCUSABLE_SELECTORS = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

export function BottomSheet({ open, onClose, title, children, className }: BottomSheetProps) {
  const sheetRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose() }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, onClose])

  useEffect(() => {
    if (open) {
      document.body.style.overflow = "hidden"
    } else {
      document.body.style.overflow = ""
    }
    return () => { document.body.style.overflow = "" }
  }, [open])

  useEffect(() => {
    if (!open) return
    const sheet = sheetRef.current
    if (!sheet) return

    const focusable = Array.from(sheet.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS))
    focusable[0]?.focus()

    const trapTab = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return
      if (focusable.length === 0) { e.preventDefault(); return }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey) {
        if (document.activeElement === first) { e.preventDefault(); last?.focus() }
      } else {
        if (document.activeElement === last) { e.preventDefault(); first?.focus() }
      }
    }
    document.addEventListener("keydown", trapTab)
    return () => document.removeEventListener("keydown", trapTab)
  }, [open])

  if (!open) return null

  return (
    <>
      <div
        className="fixed inset-0 z-50"
        style={{
          background: "var(--bg-overlay)",
          backdropFilter: "blur(2px)",
          WebkitBackdropFilter: "blur(2px)",
          animation: "fadeIn 0.24s var(--ease-out)",
        }}
        onClick={onClose}
        aria-hidden
      />
      <div
        ref={sheetRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        className={cn(
          "fixed bottom-0 left-0 right-0 z-50 outline-none flex flex-col",
          className
        )}
        style={{
          background: "var(--bg-elev-1)",
          border: "1px solid var(--line-2)",
          borderBottom: "none",
          borderRadius: "var(--rad-xl) var(--rad-xl) 0 0",
          boxShadow: "var(--shadow-3)",
          maxHeight: "86svh",
          paddingBottom: "env(safe-area-inset-bottom)",
          animation: "sheetUp 0.42s var(--ease-out)",
        }}
      >
        {/* Grabber */}
        <div className="flex justify-center pt-2.5 pb-1 flex-shrink-0">
          <div style={{ width: 36, height: 4, borderRadius: 2, background: "var(--line-3)" }} />
        </div>

        {/* Header */}
        <div
          className="flex items-center justify-between px-5 pb-3 pt-2 flex-shrink-0"
          style={{ borderBottom: "1px solid var(--line-1)" }}
        >
          {title ? (
            <span
              className="serif"
              style={{ fontSize: 22, fontWeight: 500, color: "var(--ink-1)", lineHeight: 1.2 }}
            >
              {title}
            </span>
          ) : (
            <span />
          )}
          <button
            onClick={onClose}
            className="flex items-center justify-center"
            style={{
              width: 36,
              height: 36,
              borderRadius: "50%",
              background: "var(--bg-elev-2)",
              border: "1px solid var(--line-1)",
              color: "var(--ink-3)",
              cursor: "pointer",
            }}
            aria-label="Close"
          >
            <X size={16} aria-hidden />
          </button>
        </div>

        {/* Content */}
        <div className="overflow-y-auto flex-1 px-5 py-5 scrollarea">
          {children}
        </div>
      </div>
    </>
  )
}
