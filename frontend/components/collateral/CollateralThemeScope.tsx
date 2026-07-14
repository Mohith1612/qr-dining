"use client"

import { forwardRef } from "react"
import type { CSSProperties, ReactNode } from "react"
import { themeScopeStyle } from "@/lib/collateral/theme"
import type { ThemeConfig } from "@/lib/theme/applyTheme"

// CollateralThemeScope applies a theme (preset + custom tokens) to a subtree via the
// data-theme attribute, so collateral renders theme-aware without touching <html> (which
// would change the operator's own console). styles/themes.css targets [data-theme="X"], so
// all preset tokens cascade here; custom tokens layer as inline CSS variables.
export const CollateralThemeScope = forwardRef<HTMLDivElement, {
  theme: ThemeConfig | null | undefined
  children: ReactNode
  style?: CSSProperties
  className?: string
}>(function CollateralThemeScope({ theme, children, style, className }, ref) {
  const preset = theme?.preset || "dark-luxury"
  return (
    <div
      ref={ref}
      data-theme={preset}
      className={className}
      style={{ ...themeScopeStyle(theme?.tokens), ...style }}
    >
      {children}
    </div>
  )
})
