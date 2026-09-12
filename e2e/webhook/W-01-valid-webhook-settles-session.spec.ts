import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

// LIVE SECURITY CONTROL: the route is public and unauthenticated, so signature verification is live regardless of settlement.
test.describe("W-01: Webhook signature verification discriminates", () => {
  test("valid signature reaches payment lookup while invalid signature is rejected", async () => {
    const clientSecret = process.env.PAYMENT_WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"
    const timestamp = Math.floor(Date.now() / 1000).toString()
    const body = JSON.stringify({
      id: crypto.randomUUID(),
      event: "payment.success",
      payment_ref: crypto.randomUUID(),
      amount: 150,
      currency: "INR",
      session_id: crypto.randomUUID(),
      branch_id: 1,
    })

    const sign = (secret: string) =>
      crypto.createHmac("sha256", secret).update(`${timestamp}.${body}`).digest("hex")
    const post = (signature: string) => fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Timestamp": timestamp,
        "X-Payment-Signature": signature,
      },
      body,
    })

    const accepted = await post(sign(clientSecret))
    expect(accepted.status).toBe(200)

    const rejected = await post(sign("deliberately-wrong-webhook-secret"))
    expect(rejected.status).toBe(401)
  })
})
