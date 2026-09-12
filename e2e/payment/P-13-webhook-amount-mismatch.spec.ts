import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, placeWebhook } from "../helpers/api"
import crypto from "crypto"

test.describe("P-13: Webhook amount mismatch flagged", () => {
  // VACUOUS(sig-3): accepts webhook success and failure; passes when a mismatched amount is silently accepted.
  test.fixme("webhook with amount different from initiated payment is flagged or rejected", async () => {
    const { table, menu } = await seedOrg("p13")
    const created = await createSession(table.id, "WebhookMismatch")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const paymentRef = crypto.randomUUID()

    // Initiate payment for the correct amount
    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "digital",
        idempotency_key: paymentRef,
        reference: paymentRef,
      }),
    })

    const secret = process.env.WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"

    // Send webhook with mismatched amount
    const res = await placeWebhook("stripe", {
      event: "payment.completed",
      session_id: sessionId,
      reference: paymentRef,
      amount: menu.itemPrice + 9999,
    }, secret)

    // Should be rejected or flagged — not silently accepted as full settlement
    expect([200, 400, 409, 422]).toContain(res.status)

    // If accepted (200), session should NOT be marked closed with wrong amount
    if (res.status === 200) {
      const snap = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
        headers: { "Authorization": `Bearer ${guestToken}` },
      })
      if (snap.ok) {
        const data = await snap.json()
        // session should not be closed if amount was wrong
        expect(["payment_pending", "active"]).toContain(data.session.status)
      }
    }
  })
})
