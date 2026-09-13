"use client"

import { PlatformNav } from "./PlatformNav"
import { usePlatformStore } from "@/store/platform"
import { roleLabel } from "@/lib/platform-rbac"
import { Button } from "@/components/ui/button"
import { LogOut, ShieldCheck } from "lucide-react"

type Props = {
  children: React.ReactNode
  onSignOut: () => void
}

export function PlatformShell({ children, onSignOut }: Props) {
  const { email, roles } = usePlatformStore()
  const primaryRole = roles[0]

  return (
    <div data-surface="platform" style={{ display: "flex", minHeight: "100vh", background: "var(--bg-base)", color: "var(--ink-1)" }}>
      {/* Sidebar */}
      <aside
        style={{
          width: 248,
          flexShrink: 0,
          borderRight: "1px solid var(--line-1)",
          background: "var(--bg-sunken)",
          display: "flex",
          flexDirection: "column",
          position: "sticky",
          top: 0,
          height: "100vh",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "18px 20px", borderBottom: "1px solid var(--line-1)" }}>
          <div
            style={{
              width: 32, height: 32, flexShrink: 0,
              borderRadius: "var(--rad-md)",
              background: "linear-gradient(160deg, var(--bg-elev-2), var(--bg-elev-1))",
              border: "1px solid var(--line-2)",
              display: "flex", alignItems: "center", justifyContent: "center",
            }}
          >
            <ShieldCheck size={17} style={{ color: "var(--accent)" }} aria-hidden />
          </div>
          <div style={{ display: "flex", flexDirection: "column", lineHeight: 1.2 }}>
            <span className="serif" style={{ fontSize: 16, fontWeight: 500 }}>Control Plane</span>
            <span className="eyebrow" style={{ fontSize: 9.5 }}>Platform</span>
          </div>
        </div>
        <div style={{ flex: 1, overflowY: "auto" }}>
          <PlatformNav />
        </div>
      </aside>

      {/* Main */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <header
          style={{
            height: 56,
            flexShrink: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
            gap: 16,
            padding: "0 24px",
            borderBottom: "1px solid var(--line-1)",
            background: "var(--bg-base)",
            position: "sticky",
            top: 0,
            zIndex: 20,
          }}
        >
          <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", lineHeight: 1.25 }}>
            <span style={{ fontSize: 13, color: "var(--ink-1)" }}>{email ?? "—"}</span>
            {primaryRole && (
              <span
                style={{
                  fontSize: 10.5, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.04em",
                  color: "var(--accent)",
                }}
              >
                {roleLabel(primaryRole)}
              </span>
            )}
          </div>
          <Button variant="outline" size="sm" onClick={onSignOut}>
            <LogOut size={14} aria-hidden /> Sign out
          </Button>
        </header>

        <main style={{ flex: 1, padding: "28px 32px", maxWidth: 1280, width: "100%" }}>
          {children}
        </main>
      </div>
    </div>
  )
}
