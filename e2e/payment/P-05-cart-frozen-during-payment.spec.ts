import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-05: Cart mutation rejected during payment_pending", () => {
  test("adding items after payment initiated returns 409 PAYMENT_IN_PROGRESS", async () => {
    const { table, menu } = await seedOrg("p05")
    const created = await createSession(table.id, "FrozenGuest")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId)

    // Initiate payment
    const payRes = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        amount: menu.itemPrice,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect([200, 201]).toContain(payRes.status)

    // Attempt to add cart item
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect([409]).toContain(cartRes.status)
    if (cartRes.status === 409) {
      const err = await cartRes.json()
      expect(err.code).toBe("PAYMENT_IN_PROGRESS")
    }
  })
})
