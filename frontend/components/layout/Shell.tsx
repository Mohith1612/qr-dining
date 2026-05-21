"use client"

import { ReactNode } from "react"

interface ShellProps {
  children: ReactNode
  nav?: ReactNode
  topBar?: ReactNode
  banner?: ReactNode
}

export function Shell({ children, nav, topBar, banner }: ShellProps) {
  return (
    <div
      className="flex flex-col min-h-svh"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      {topBar}
      {banner}
      <main className="flex-1 overflow-y-auto" style={{ paddingBottom: nav ? "calc(4rem + env(safe-area-inset-bottom))" : "env(safe-area-inset-bottom)" }}>
        {children}
      </main>
      {nav && (
        <div
          className="fixed bottom-0 left-0 right-0 z-40"
          style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
        >
          {nav}
        </div>
      )}
    </div>
  )
}
