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
      className="flex flex-col h-svh overflow-hidden"
      style={{ backgroundColor: "var(--bg-base)", color: "var(--ink-1)", fontFamily: "var(--font-body)" }}
    >
      {topBar}
      {banner}
      <main
        className="flex-1 overflow-y-auto scrollarea"
        style={{ paddingBottom: nav ? "calc(84px + env(safe-area-inset-bottom))" : "env(safe-area-inset-bottom)" }}
      >
        {children}
      </main>
      {nav && (
        <div className="fixed bottom-0 left-0 right-0 z-40">
          {nav}
        </div>
      )}
    </div>
  )
}
