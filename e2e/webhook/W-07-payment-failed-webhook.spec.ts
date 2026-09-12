import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, placeWebhook } from "../helpers/api"
import crypto from "crypto"

test.describe("W-07: payment.failed webhook reverts session to active or payment_pending", () => {
  // VACUOUS(sig-3): accepts 404 from the failed webhook and conditionally checks state; passes when reset is absent.
  test.fixme("payment.failed resets payment state without closing session", async () => {
    const { table, menu } = await seedOrg("w07")
    const created = await createSession(table.id, "FailWebhook")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const ref = crypto.randomUUID()
    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "digital",
        idempotency_key: ref,
        reference: ref,
      }),
    })

    const secret = process.env.WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"
    const failRes = await placeWebhook("stripe", {
      event: "payment.failed",
      session_id: sessionId,
      reference: ref,
      reason: "insufficient_funds",
    }, secret)

    expect([200, 202, 404]).toContain(failRes.status)

    // Session should remain accessible (not closed due to payment failure)
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["active", "payment_pending"]).toContain(snap.session.status)
    }
  })
})
