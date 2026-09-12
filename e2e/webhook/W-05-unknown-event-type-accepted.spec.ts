import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { placeWebhook } from "../helpers/api"

// UNIMPLEMENTED/OUT OF SCOPE (P5): payment method is a staff signal, not provider settlement.
test.describe.skip("W-05: Unknown webhook event type accepted gracefully", () => {
  // VACUOUS(sig-3): accepts success and rejection for an event claimed accepted; passes when unknown events are rejected.
  test.fixme("unrecognised event type returns 200 without error (no-op)", async () => {
    const secret = process.env.PAYMENT_WEBHOOK_SECRET_STRIPE ?? "test-webhook-secret"

    const res = await placeWebhook("stripe", {
      event: "payment.future_unknown_event",
      data: { foo: "bar" },
    }, secret)

    // Should be accepted with 200/202 (no-op) — not 500
    expect([200, 202, 400]).toContain(res.status)
    expect(res.status).not.toBe(500)
  })
})
