import { ApiError } from "@/lib/api/client"
import type { PaymentStatus, StaffRole } from "@/types/api"

/** Mirrors handlers.maxRecoveryReasonLen. The reason is stored in the audit
 *  trail, so it is bounded server-side; enforcing it here keeps a waiter from
 *  losing a long note to a 400. */
export const MAX_RECOVERY_REASON_LEN = 500

/** Payment statuses that still hold a session's payment_pending freeze.
 *  Mirrors services.isTerminalPaymentStatus (inverted). */
const TERMINAL_PAYMENT_STATUSES: PaymentStatus[] = [
  "completed", "failed", "cancelled", "refunded", "partially_refunded",
]

export function isNonTerminalPayment(status: PaymentStatus): boolean {
  return !TERMINAL_PAYMENT_STATUSES.includes(status)
}

/** Waiters, managers and owners may cancel a payment — the same roles that may
 *  settle one. Mirrors PaymentHandler.Cancel. */
export function canCancelPayment(role: StaffRole | null): boolean {
  return role === "owner" || role === "manager" || role === "waiter"
}

/** Managers and owners only: force-close can discard an unpaid bill, so it is a
 *  narrower permission than cancelling a payment. Mirrors SessionHandler.ForceClose. */
export function canForceCloseSession(role: StaffRole | null): boolean {
  return role === "owner" || role === "manager"
}

export type RecoveryAction = "cancel-payment" | "force-close"

// Codes the two recovery handlers actually return, mapped to something a waiter
// can act on mid-service. Keyed on code (never message) per the APIError contract.
// The shared friendlyErrorMessage() map is guest-facing copy — "Your table
// session has paused" is wrong for a manager closing someone else's table.
const CANCEL_MESSAGES: Record<string, string> = {
  VALIDATION_ERROR:           "Add a reason before cancelling.",
  UNAUTHORIZED:               "Your staff session expired. Sign in again.",
  FORBIDDEN:                  "You don't have permission to cancel this payment.",
  PAYMENT_NOT_FOUND:          "That payment is no longer on this branch.",
  INVALID_PAYMENT_TRANSITION: "That payment already moved — it was settled or cancelled by someone else.",
}

const FORCE_CLOSE_MESSAGES: Record<string, string> = {
  VALIDATION_ERROR:       "Add a reason before closing the table.",
  UNAUTHORIZED:           "Your staff session expired. Sign in again.",
  FORBIDDEN:              "Only managers and owners can force-close a table.",
  ORGANIZATION_SUSPENDED: "This restaurant is suspended. Contact support.",
  BRANCH_SUSPENDED:       "This branch is suspended. Contact support.",
  SESSION_NOT_FOUND:      "That table's session no longer exists.",
  SESSION_CLOSED:         "That table is already closed — someone got there first.",
}

const FALLBACK: Record<RecoveryAction, string> = {
  "cancel-payment": "Couldn't cancel the payment. Please try again.",
  "force-close":    "Couldn't close the table. Please try again.",
}

export function recoveryErrorMessage(err: unknown, action: RecoveryAction): string {
  if (!(err instanceof ApiError)) return FALLBACK[action]
  const map = action === "cancel-payment" ? CANCEL_MESSAGES : FORCE_CLOSE_MESSAGES
  return map[err.code] ?? FALLBACK[action]
}

/** True when the failure means our view is stale and a refetch will fix it —
 *  the row we acted on has already moved. */
export function isStaleRecoveryError(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 404 || err.status === 409)
}
