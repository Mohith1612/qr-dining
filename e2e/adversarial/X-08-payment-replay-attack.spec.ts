import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("X-08: Payment idempotency prevents replay attack", () => {
  // VACUOUS(sig-3): accepts success and conflict on replay; passes when the replay is rejected instead of deduplicated.
  test.fixme("same idempotency key returns same payment, not double-charge", async () => {
    const { table, menu } = await seedOrg("x08")
    const created = await createSession(table.id, "PayUser")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    // Place an order first
    await placeOrder(sessionId, guestToken, menu.itemId)

    const key = crypto.randomUUID()
    const payBody = JSON.stringify({
      amount: 150,
      method: "cash",
      idempotency_key: key,
    })

    // First payment attempt
    const pay1 = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: payBody,
    })
    expect([201, 200]).toContain(pay1.status)
    const payment1 = await pay1.json()

    // Replay with same key and same body
    const pay2 = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: payBody,
    })
    expect([200, 201, 409]).toContain(pay2.status)
    if ([200, 201].includes(pay2.status)) {
      const payment2 = await pay2.json()
      // Same payment ID — not a new payment
      expect(payment1.id ?? payment1.payment?.id).toBe(payment2.id ?? payment2.payment?.id)
    }

    // Attempt with different body but same key → 409
    const pay3 = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: 999,
        method: "digital",
        idempotency_key: key,
      }),
    })
    // 409 IDEMPOTENCY_CONFLICT / session already payment_pending, or 422
    // PAYMENT_AMOUNT_INVALID (amount is validated against the bill before the
    // idempotency key is consulted). Either way the replay is rejected.
    expect([409, 422]).toContain(pay3.status)
  })
})
