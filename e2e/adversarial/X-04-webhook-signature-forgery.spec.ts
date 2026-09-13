import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

// LIVE SECURITY CONTROL: the route is public and unauthenticated, so signature verification is live regardless of settlement.
test.describe("X-04: Webhook signature forgery rejected", () => {
  test("webhook with invalid signature returns 401", async () => {
    const payload = JSON.stringify({
      event: "payment.completed",
      payment_id: crypto.randomUUID(),
      amount: 100,
    })
    const ts = Math.floor(Date.now() / 1000).toString()
    const badSig = "deadbeefdeadbeef0000000000000000deadbeefdeadbeef0000000000000000"

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Timestamp": ts,
        "X-Payment-Signature": badSig,
      },
      body: payload,
    })
    expect(res.status).toBe(401)
  })

  test("webhook with missing timestamp returns 400/401", async () => {
    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Signature": "abc123",
      },
      body: JSON.stringify({ event: "test" }),
    })
    expect(res.status).toBe(401)
  })

  test("webhook with stale timestamp rejected", async () => {
    const staleTs = (Math.floor(Date.now() / 1000) - 400).toString() // 6+ minutes old
    const payload = JSON.stringify({ event: "test" })
    const secret = process.env.PAYMENT_WEBHOOK_SECRET_STRIPE ?? "test-secret"
    const sig = crypto.createHmac("sha256", secret).update(`${staleTs}.${payload}`).digest("hex")

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Timestamp": staleTs,
        "X-Payment-Signature": sig,
      },
      body: payload,
    })
    expect(res.status).toBe(401)
  })
})
