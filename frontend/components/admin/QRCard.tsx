"use client"

import { QRCodeSVG } from "qrcode.react"
import { buildQRUrl } from "@/lib/qr"
import { useTenant } from "@/providers/TenantProvider"
import type { Table } from "@/types/api"

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
        background: "#181410",
        border: "1px solid rgba(201,168,118,0.3)",
        borderRadius: 6,
        padding: 4,
        display: "inline-flex",
        flexShrink: 0,
      }}
    >
      <QRCodeSVG
        value={url}
        size={size}
        fgColor="#F4E8D1"
        bgColor="#181410"
        level="M"
      />
    </div>
  )
}
