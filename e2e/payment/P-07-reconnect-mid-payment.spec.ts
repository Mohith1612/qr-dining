import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchSnapshot } from "../helpers/api"
import crypto from "crypto"

test.describe("P-07: Reconnect mid-payment — no duplicate payment", () => {
  // VACUOUS(sig-7): claims reconnect behavior but uses HTTP requests only; passes when browser reconnection duplicates payment.
  test.fixme("snapshot after payment initiation shows pending state without duplicate", async () => {
    const { table, menu } = await seedOrg("p07")
    const created = await createSession(table.id, "ReconnPay")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    // Initiate payment
    const key = crypto.randomUUID()
    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ amount: menu.itemPrice, method: "cash", idempotency_key: key }),
    })

    // Simulate reconnect: fetch snapshot as if reconnecting
    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(["active", "payment_pending"]).toContain(snap.session.status)

    // Retry payment with same idempotency key — should NOT create duplicate
    const retryRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ amount: menu.itemPrice, method: "cash", idempotency_key: key }),
    })
    expect([200, 201, 409]).toContain(retryRes.status)
  })
})
