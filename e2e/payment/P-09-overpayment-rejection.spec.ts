import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-09: Overpayment rejection", () => {
  test("payment exceeding bill total is rejected", async () => {
    const { table, menu } = await seedOrg("p09")
    const created = await createSession(table.id, "Overpayer")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    const overpayAmount = menu.itemPrice * 10

    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: overpayAmount,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })

    expect([400, 409, 422]).toContain(payRes.status)
    if (payRes.status === 400 || payRes.status === 409 || payRes.status === 422) {
      const err = await payRes.json()
      expect(err.code).toBeDefined()
    }
  })
})
