"use client"

import { useState, type CSSProperties, type ReactNode } from "react"
import { Loader2 } from "lucide-react"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { MAX_RECOVERY_REASON_LEN, type RecoveryAction } from "@/lib/staff-recovery"

interface RecoveryActionSheetProps {
  action: RecoveryAction
  title: string
  /** What the action does in the guest's terms, above the reason field. */
  consequence: ReactNode
  /** Force-close only: restated once more on the confirmation stage. */
  confirmSummary?: ReactNode
  submitLabel: string
  onClose: () => void
  /** Resolve to close the sheet; reject to keep it open for another try. */
  onSubmit: (reason: string) => Promise<void>
}

/**
 * The reason-then-act sheet behind both staff recovery actions.
 *
 * The reason is mandatory because it is the only thing the audit entry records
 * about *why* a bill was withdrawn or a table ended — an empty or defaulted
 * string would make the trail useless. So the submit button stays disabled
 * until there is one, and no request is ever sent without it.
 *
 * Force-close additionally takes a confirmation stage: it discards an unpaid
 * bill and revokes every guest credential at the table, and neither is
 * recoverable. Cancelling a payment is — the guest can request the bill again —
 * so that one submits straight from the reason stage rather than charging a
 * waiter mid-service for a second tap that buys nothing.
 */
export function RecoveryActionSheet({
  action, title, consequence, confirmSummary, submitLabel, onClose, onSubmit,
}: RecoveryActionSheetProps) {
  const requiresConfirm = action === "force-close"
  const [stage, setStage] = useState<"reason" | "confirm">("reason")
  const [reason, setReason] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const trimmed = reason.trim()
  const hasReason = trimmed.length > 0
  const nearLimit = reason.length > MAX_RECOVERY_REASON_LEN - 50

  async function handleSubmit() {
    if (!hasReason || submitting) return
    if (requiresConfirm && stage === "reason") {
      setStage("confirm")
      return
    }
    setSubmitting(true)
    try {
      await onSubmit(trimmed)
      onClose()
    } catch {
      // Parent surfaces the error; keep the sheet open with the reason intact.
      setSubmitting(false)
    }
  }

  return (
    <BottomSheet open onClose={onClose} title={title}>
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {stage === "reason" ? (
          <>
            <div style={{ fontSize: 13, lineHeight: 1.55, color: "var(--ink-2)" }}>{consequence}</div>

            <div>
              <label htmlFor="recovery-reason" style={labelStyle}>
                Reason (required)
              </label>
              <textarea
                id="recovery-reason"
                value={reason}
                rows={3}
                autoFocus
                maxLength={MAX_RECOVERY_REASON_LEN}
                onChange={(e) => setReason(e.target.value)}
                placeholder={action === "cancel-payment"
                  ? "e.g. Guest asked to keep ordering"
                  : "e.g. Table left without settling"}
                style={textareaStyle}
              />
              <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginTop: 6 }}>
                <span style={{ fontSize: 11, color: "var(--ink-3)" }} aria-live="polite">
                  {hasReason ? "Saved to the audit log." : "A reason is required — it goes on the audit log."}
                </span>
                <span
                  className="mono"
                  style={{ fontSize: 11, color: nearLimit ? "var(--warn)" : "var(--ink-4)" }}
                >
                  {reason.length}/{MAX_RECOVERY_REASON_LEN}
                </span>
              </div>
            </div>
          </>
        ) : (
          <div
            style={{
              fontSize: 13, lineHeight: 1.55, color: "var(--ink-1)",
              background: "var(--alert-soft)", border: "1px solid var(--alert)",
              borderRadius: "var(--rad-md)", padding: "12px 14px",
            }}
            role="alert"
          >
            {confirmSummary}
          </div>
        )}

        <div style={{ display: "flex", gap: 8 }}>
          {stage === "confirm" && (
            <button
              type="button"
              onClick={() => setStage("reason")}
              disabled={submitting}
              className="press"
              style={{ ...actionButtonStyle, flex: "0 0 auto", paddingInline: 18, background: "var(--bg-elev-2)", color: "var(--ink-2)", border: "1px solid var(--line-2)" }}
            >
              Back
            </button>
          )}
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!hasReason || submitting}
            className="press"
            style={{
              ...actionButtonStyle,
              flex: 1,
              background: requiresConfirm ? "var(--alert)" : "var(--accent)",
              color: requiresConfirm ? "white" : "var(--accent-ink)",
              cursor: !hasReason || submitting ? "not-allowed" : "pointer",
              opacity: !hasReason || submitting ? 0.55 : 1,
            }}
          >
            {submitting && <Loader2 size={14} className="animate-spin" />}
            {requiresConfirm && stage === "reason" ? "Continue" : submitLabel}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}

const labelStyle: CSSProperties = {
  display: "block", fontSize: 12, fontWeight: 600,
  color: "var(--ink-3)", marginBottom: 6,
}

const textareaStyle: CSSProperties = {
  width: "100%", fontSize: 14, padding: "8px 12px",
  borderRadius: "var(--rad-md)", border: "1px solid var(--line-2)",
  background: "var(--bg-elev-2)", color: "var(--ink-1)", resize: "none",
}

const actionButtonStyle: CSSProperties = {
  height: 44, borderRadius: "var(--rad-md)", border: "none",
  fontSize: 14, fontWeight: 600, cursor: "pointer",
  display: "flex", alignItems: "center", justifyContent: "center", gap: 8,
}
