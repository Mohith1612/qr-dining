"use client"

import { ReactNode, useEffect, useRef } from "react"
import { AnimatePresence, motion } from "framer-motion"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"
import { springGentle, prefersReduced } from "@/lib/motion"

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  className?: string
}

export function BottomSheet({ open, onClose, title, children, className }: BottomSheetProps) {
  const sheetRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, onClose])

  useEffect(() => {
    if (open) {
      document.body.style.overflow = "hidden"
      sheetRef.current?.focus()
    } else {
      document.body.style.overflow = ""
    }
    return () => { document.body.style.overflow = "" }
  }, [open])

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            key="backdrop"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: prefersReduced ? 0 : 0.2 }}
            className="fixed inset-0 z-50"
            style={{
              backgroundColor: "var(--color-overlay)",
              backdropFilter: "blur(4px)",
              WebkitBackdropFilter: "blur(4px)",
            }}
            onClick={onClose}
            aria-hidden
          />
          <motion.div
            key="sheet"
            ref={sheetRef}
            role="dialog"
            aria-modal="true"
            aria-label={title}
            tabIndex={-1}
            initial={prefersReduced ? {} : { y: "100%" }}
            animate={{ y: 0 }}
            exit={prefersReduced ? {} : { y: "100%" }}
            transition={springGentle}
            className={cn(
              "fixed bottom-0 left-0 right-0 z-50 rounded-t-3xl outline-none",
              "max-h-[92svh] flex flex-col",
              className
            )}
            style={{
              backgroundColor: "var(--color-surface)",
              boxShadow: "var(--shadow-elevated)",
              paddingBottom: "env(safe-area-inset-bottom)",
            }}
          >
            {/* Handle bar */}
            <div className="flex justify-center pt-3 pb-1 flex-shrink-0">
              <div
                className="w-12 h-1.5 rounded-full"
                style={{ backgroundColor: "var(--color-border)" }}
              />
            </div>

            {/* Header */}
            <div
              className="flex items-center justify-between px-5 pb-4 pt-1 border-b flex-shrink-0"
              style={{ borderColor: "var(--color-border)" }}
            >
              {title ? (
                <span
                  className="text-xl font-medium leading-snug"
                  style={{
                    fontFamily: "var(--font-display)",
                    color: "var(--color-text)",
                  }}
                >
                  {title}
                </span>
              ) : (
                <span />
              )}
              <button
                onClick={onClose}
                className="p-2 rounded-full min-h-[44px] min-w-[44px] flex items-center justify-center transition-opacity active:opacity-60"
                style={{ color: "var(--color-text-muted)" }}
                aria-label="Close"
              >
                <X className="size-5" aria-hidden />
              </button>
            </div>

            {/* Content */}
            <div className="overflow-y-auto flex-1 px-5 py-5">
              {children}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}
