"use client"

import { useState } from "react"
import { toast } from "sonner"
import { BottomSheet } from "./BottomSheet"
import { customersApi } from "@/lib/api/customers"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"

interface Props {
  sessionId: string
  participantId: number
  onComplete: () => void
}

export function CustomerOptIn({ sessionId, participantId, onComplete }: Props) {
  const [phone, setPhone] = useState("")
  const [name, setName] = useState("")
  const [saving, setSaving] = useState(false)

  async function handleSave() {
    setSaving(true)
    try {
      await customersApi.link(sessionId, "+91" + phone, name, participantId)
      toast.success("We'll remember you next time!")
      onComplete()
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't save. Please try again.")
    } finally {
      setSaving(false)
    }
  }

  const inputStyle: React.CSSProperties = {
    width: "100%",
    padding: "11px 14px",
    borderRadius: "var(--rad-md)",
    background: "var(--bg-elev-2)",
    border: "1px solid var(--line-2)",
    color: "var(--ink-1)",
    fontSize: 14,
    outline: "none",
    boxSizing: "border-box",
  }

  return (
    <BottomSheet open onClose={onComplete} title="Come back anytime">
      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        <p style={{ color: "var(--ink-2)", fontSize: 14, lineHeight: 1.65, margin: 0 }}>
          Share your number and we&apos;ll remember your preferences for next time.
        </p>

        <div>
          <span className="eyebrow" style={{ display: "block", marginBottom: 6 }}>Mobile number</span>
          <div style={{ display: "flex", gap: 8 }}>
            <div style={{
              padding: "11px 12px",
              borderRadius: "var(--rad-md)",
              background: "var(--bg-elev-2)",
              border: "1px solid var(--line-2)",
              color: "var(--ink-2)",
              fontSize: 14,
              flexShrink: 0,
              display: "flex",
              alignItems: "center",
            }}>
              🇮🇳 +91
            </div>
            <input
              type="tel"
              inputMode="numeric"
              maxLength={10}
              placeholder="98765 43210"
              value={phone}
              onChange={(e) => setPhone(e.target.value.replace(/\D/g, ""))}
              style={{ ...inputStyle, flex: 1, width: "auto" }}
            />
          </div>
        </div>

        <div>
          <span className="eyebrow" style={{ display: "block", marginBottom: 6 }}>Your name (optional)</span>
          <input
            type="text"
            placeholder="How should we call you?"
            value={name}
            onChange={(e) => setName(e.target.value)}
            style={inputStyle}
          />
        </div>

        <button
          onClick={handleSave}
          disabled={phone.length !== 10 || saving}
          className="press btn-primary"
          style={{
            width: "100%",
            height: 54,
            borderRadius: 16,
            fontSize: 15,
            fontWeight: 600,
            opacity: phone.length !== 10 || saving ? 0.5 : 1,
            cursor: phone.length !== 10 || saving ? "not-allowed" : "pointer",
            transition: "opacity var(--dur-fast) var(--ease)",
          }}
        >
          {saving ? "Saving…" : "Remember me"}
        </button>

        <button
          onClick={onComplete}
          style={{
            background: "none",
            border: "none",
            color: "var(--ink-3)",
            fontSize: 13,
            cursor: "pointer",
            padding: "4px 0",
          }}
        >
          No thanks
        </button>
      </div>
    </BottomSheet>
  )
}
