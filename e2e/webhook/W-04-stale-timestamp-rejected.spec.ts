import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

test.describe("W-04: Stale webhook timestamp rejected", () => {
  test("webhook with timestamp older than 5 minutes is rejected", async () => {
    const secret = process.env.WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"
    const staleTs = Math.floor((Date.now() - 10 * 60 * 1000) / 1000).toString()
    const body = JSON.stringify({ event: "payment.completed", amount: 100 })
    const sig = crypto.createHmac("sha256", secret).update(`${staleTs}.${body}`).digest("hex")

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Webhook-Timestamp": staleTs,
        "X-Webhook-Signature": `v1=${sig}`,
      },
      body,
    })
    expect([400, 401, 422]).toContain(res.status)
  })
})
