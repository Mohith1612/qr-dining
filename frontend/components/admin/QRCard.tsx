"use client"

import dynamic from "next/dynamic"
import { buildQRUrl } from "@/lib/qr"
import { useTenant } from "@/providers/TenantProvider"
import type { Table } from "@/types/api"

const QRCodeSVG = dynamic(() => import("qrcode.react").then((m) => m.QRCodeSVG), { ssr: false })

interface Props {
  table: Table
  size?: number
}

export function QRCard({ table, size = 64 }: Props) {
  const { slug } = useTenant()
  const url = buildQRUrl(table.qr_code_token, slug)

  return (
    <div
      style={{
        // Theme-aware and transparent: the QR takes the tenant's ink colour and
        // sits on whatever surface it's placed on, rather than a fixed
        // black-on-white block. `fill="var(--…)"` resolves against the themed
        // document tokens; a quiet-zone padding keeps it scannable.
        background: "transparent",
        border: "1px solid var(--line-2)",
        borderRadius: 6,
        padding: 5,
        display: "inline-flex",
        flexShrink: 0,
      }}
    >
      <QRCodeSVG
        value={url}
        size={size}
        fgColor="var(--ink-1)"
        bgColor="transparent"
        level="M"
      />
    </div>
  )
}
