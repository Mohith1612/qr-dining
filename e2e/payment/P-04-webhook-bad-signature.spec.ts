import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

// LIVE SECURITY CONTROL: the route is public and unauthenticated, so signature verification is live regardless of settlement.
test.describe("P-04: Webhook bad signature rejected", () => {
  test("tampered webhook body returns 401", async () => {
    const secret = process.env.PAYMENT_WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"
    const ts = Math.floor(Date.now() / 1000).toString()
    const body = JSON.stringify({ event: "payment.completed", amount: 100 })
    const sig = crypto.createHmac("sha256", secret).update(`${ts}.${body}`).digest("hex")

    // Tamper the body after signing
    const tamperedBody = JSON.stringify({ event: "payment.completed", amount: 999999 })

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Timestamp": ts,
        "X-Payment-Signature": sig,
      },
      body: tamperedBody,
    })
    expect(res.status).toBe(401)
  })
})
