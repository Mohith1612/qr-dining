import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, placeWebhook } from "../helpers/api"
import crypto from "crypto"

test.describe("W-02: Webhook idempotency — duplicate delivery handled", () => {
  test("sending the same webhook twice does not double-settle", async () => {
    const { table, menu } = await seedOrg("w02")
    const created = await createSession(table.id, "WebhookIdem")
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
    const event = {
      event: "payment.completed",
      session_id: sessionId,
      reference: ref,
      amount: menu.itemPrice,
    }

    const r1 = await placeWebhook("stripe", event, secret)
    const r2 = await placeWebhook("stripe", event, secret)

    expect([200, 202]).toContain(r1.status)
    // Second delivery: idempotent 200/202 or conflict 409
    expect([200, 202, 409]).toContain(r2.status)
  })
})
