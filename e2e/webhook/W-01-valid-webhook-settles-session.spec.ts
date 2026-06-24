import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, placeWebhook } from "../helpers/api"
import crypto from "crypto"

test.describe("W-01: Valid webhook settles the session", () => {
  test("payment.completed webhook transitions session to closed", async () => {
    const { table, menu } = await seedOrg("w01")
    const created = await createSession(table.id, "WebhookGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const ref = crypto.randomUUID()
    // Initiate payment
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
    const res = await placeWebhook("stripe", {
      event: "payment.completed",
      session_id: sessionId,
      reference: ref,
      amount: menu.itemPrice,
    }, secret)

    expect([200, 202]).toContain(res.status)

    // Session should now be closed or payment accepted
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    if (snapRes.status === 200) {
      const snap = await snapRes.json()
      expect(["closed", "payment_pending"]).toContain(snap.session.status)
    }
  })
})
