"use client"

import { ThemeProvider as NextThemesProvider } from "next-themes"

const THEME_ATTRIBUTE = "data-theme"
const DEFAULT_THEME = "warm-cafe"
const THEMES = ["warm-cafe", "modern-minimal", "dark-luxury", "vibrant"] as const

export type AppTheme = (typeof THEMES)[number]

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextThemesProvider
      attribute={THEME_ATTRIBUTE}
      defaultTheme={DEFAULT_THEME}
      themes={THEMES as unknown as string[]}
      enableSystem={false}
      storageKey="qr-dining-theme"
    >
      {children}
    </NextThemesProvider>
  )
}

export { THEMES }
