import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { placeWebhook } from "../helpers/api"
import crypto from "crypto"

// UNIMPLEMENTED/OUT OF SCOPE (P5): there is no merchant provider settlement integration.
test.describe.skip("P-03: Webhook replay idempotency", () => {
  // VACUOUS(sig-3): accepts success and multiple failure statuses; passes when no webhook is processed.
  test.fixme("sending same signed webhook 3 times processes once", async () => {
    const secret = process.env.PAYMENT_WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"
    const externalEventId = crypto.randomUUID()
    const event = {
      id: externalEventId,
      event: "payment.completed",
      payment_intent_id: crypto.randomUUID(),
      amount: 15000,
    }

    const ts = Math.floor(Date.now() / 1000).toString()
    const body = JSON.stringify(event)
    const sigPayload = `${ts}.${body}`
    const sig = crypto.createHmac("sha256", secret).update(sigPayload).digest("hex")

    const results = await Promise.all(
      Array.from({ length: 3 }, () =>
        fetch(`${API_URL}/webhooks/payments/stripe`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Payment-Timestamp": ts,
            "X-Payment-Signature": sig,
          },
          body,
        })
      )
    )

    const statuses = results.map((r) => r.status)
    // First one processes (200/404 if payment not found), others are idempotent
    // 200 = processed, 404 = payment not found, 409 = already processed
    expect(statuses.every((s) => [200, 204, 400, 404, 409].includes(s))).toBe(true)
  })
})
