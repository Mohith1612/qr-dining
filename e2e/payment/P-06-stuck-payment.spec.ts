import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-06: Stuck payment_pending handling", () => {
  test("payment initiated enters payment_pending state", async () => {
    const { table, menu } = await seedOrg("p06")
    const created = await createSession(table.id, "StuckPay")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "digital",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect([200, 201]).toContain(payRes.status)

    // Verify session is in payment_pending
    const snapRes = await fetch(`${API_URL}/sessions/${sessionId}/snapshot`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(snapRes.status).toBe(200)
    const snap = await snapRes.json()
    expect(snap.session.status).toBe("payment_pending")
  })
})
