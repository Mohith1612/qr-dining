"use client"

import { QRCodeSVG } from "qrcode.react"
import { buildQRUrl } from "@/lib/qr"
import type { Table } from "@/types/api"

interface Props {
  table: Table
  tenantSlug?: string | null
}

export function PrintTemplate({ table, tenantSlug }: Props) {
  const url = buildQRUrl(table.qr_code_token, tenantSlug)

  return (
    <div id="print-template" style={{ display: "none" }}>
      <style>{`
        @media print {
          body > *:not(#print-template) { display: none !important; }
          #print-template { display: flex !important; }
        }
        @page {
          size: A5;
          margin: 0;
        }
      `}</style>

      <div
        style={{
          width: "148mm",
          height: "210mm",
          background: "#0E0C09",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          position: "relative",
          overflow: "hidden",
        }}
      >
        {/* Radial brass glow at top */}
        <div
          style={{
            position: "absolute",
            inset: 0,
            background:
              "radial-gradient(ellipse 80% 50% at 50% 0%, rgba(201,168,118,0.16), transparent)",
            pointerEvents: "none",
          }}
        />

        {/* Card */}
        <div
          style={{
            width: "80mm",
            padding: "12mm",
            background: "#181410",
            border: "1px solid #C9A876",
            borderRadius: 14,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 6,
            position: "relative",
            zIndex: 1,
          }}
        >
          {/* Wordmark */}
          <div
            style={{
              color: "#C9A876",
              fontSize: 11,
              letterSpacing: "0.35em",
              fontWeight: 600,
              textTransform: "uppercase",
            }}
          >
            Maison Saffron
          </div>

          <div style={{ width: "60%", height: 1, background: "#5C5340", margin: "2mm 0" }} />

          <QRCodeSVG
            value={url}
            size={140}
            fgColor="#F4E8D1"
            bgColor="#181410"
            level="M"
          />

          <div style={{ width: "60%", height: 1, background: "#5C5340", margin: "2mm 0" }} />

          <div
            style={{
              color: "#C9A876",
              fontSize: 9,
              letterSpacing: "0.4em",
              fontWeight: 500,
              textTransform: "uppercase",
            }}
          >
            TABLE
          </div>

          {/* Table identifier — large Cormorant Garamond */}
          <div
            style={{
              fontFamily: "var(--font-cormorant-garamond), Georgia, serif",
              color: "#F4E8D1",
              fontSize: 52,
              fontWeight: 500,
              letterSpacing: "-0.02em",
              lineHeight: 1,
            }}
          >
            {table.identifier}
          </div>

          <div
            style={{
              color: "#8A7E63",
              fontSize: 8,
              letterSpacing: "0.3em",
              textTransform: "uppercase",
              marginTop: "2mm",
            }}
          >
            Scan to order
          </div>
        </div>
      </div>
    </div>
  )
}
