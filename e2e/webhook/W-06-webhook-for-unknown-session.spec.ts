import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { placeWebhook } from "../helpers/api"
import crypto from "crypto"

test.describe("W-06: Webhook referencing unknown session handled safely", () => {
  // VACUOUS(sig-3): accepts success and multiple failures; passes when the handler processes the unknown session incorrectly.
  test.fixme("webhook for non-existent session_id returns 404 or is no-op", async () => {
    const secret = process.env.WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"

    const res = await placeWebhook("stripe", {
      event: "payment.completed",
      session_id: crypto.randomUUID(),
      reference: crypto.randomUUID(),
      amount: 1000,
    }, secret)

    // Should not 500; either gracefully reject or no-op
    expect([200, 202, 404, 422]).toContain(res.status)
    expect(res.status).not.toBe(500)
  })
})
