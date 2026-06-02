import type { Metadata, Viewport } from "next"
import { Geist, Geist_Mono, Cormorant_Garamond } from "next/font/google"
import "./globals.css"
import "@/styles/themes.css"
import { Providers } from "@/providers/Providers"

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
})

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
})

const cormorantGaramond = Cormorant_Garamond({
  variable: "--font-cormorant-garamond",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
})

export const metadata: Metadata = {
  title: "QR Dining",
  description: "Dine together, order together",
  manifest: "/manifest.json",
}

export const viewport: Viewport = {
  themeColor: "#0E0C09",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${geistSans.variable} ${geistMono.variable} ${cormorantGaramond.variable} antialiased`}>
        <Providers>
          <main>{children}</main>
        </Providers>
      </body>
    </html>
  )
}
