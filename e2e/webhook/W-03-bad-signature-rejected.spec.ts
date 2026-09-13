import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import crypto from "crypto"

// LIVE SECURITY CONTROL: the route is public and unauthenticated, so signature verification is live regardless of settlement.
test.describe("W-03: Bad webhook signature rejected", () => {
  test("unsigned webhook returns 400 or 401", async () => {
    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ event: "payment.completed", amount: 100 }),
    })
    expect(res.status).toBe(401)
  })

  test("wrong secret produces rejection", async () => {
    const ts = Math.floor(Date.now() / 1000).toString()
    const body = JSON.stringify({ event: "payment.completed", amount: 100 })
    const wrongSig = crypto.createHmac("sha256", "wrong-secret").update(`${ts}.${body}`).digest("hex")

    const res = await fetch(`${API_URL}/webhooks/payments/stripe`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Payment-Timestamp": ts,
        "X-Payment-Signature": wrongSig,
      },
      body,
    })
    expect(res.status).toBe(401)
  })
})
