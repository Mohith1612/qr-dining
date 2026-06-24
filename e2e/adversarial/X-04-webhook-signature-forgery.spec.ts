import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

test.describe("X-04: Webhook signature forgery rejected", () => {
  test("webhook with invalid signature returns 401", async () => {
    const payload = JSON.stringify({
      event: "payment.completed",
      payment_id: crypto.randomUUID(),
      amount: 100,
    })
    const ts = Math.floor(Date.now() / 1000).toString()
    const badSig = "v1=deadbeefdeadbeef0000000000000000deadbeefdeadbeef0000000000000000"

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Webhook-Timestamp": ts,
        "X-Webhook-Signature": badSig,
      },
      body: payload,
    })
    expect([401, 400]).toContain(res.status)
  })

  test("webhook with missing timestamp returns 400/401", async () => {
    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Webhook-Signature": "v1=abc123",
      },
      body: JSON.stringify({ event: "test" }),
    })
    expect([400, 401]).toContain(res.status)
  })

  test("webhook with stale timestamp rejected", async () => {
    const staleTs = (Math.floor(Date.now() / 1000) - 400).toString() // 6+ minutes old
    const payload = JSON.stringify({ event: "test" })
    const secret = process.env.WEBHOOK_SECRET_STRIPE ?? "test-secret"
    const sig = crypto.createHmac("sha256", secret).update(`${staleTs}.${payload}`).digest("hex")

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Webhook-Timestamp": staleTs,
        "X-Webhook-Signature": `v1=${sig}`,
      },
      body: payload,
    })
    expect([400, 401]).toContain(res.status)
  })
})
