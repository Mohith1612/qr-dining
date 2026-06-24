import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-12: payment_pending blocks new order placement", () => {
  test("placing a new order while payment_pending returns 409", async () => {
    const { table, menu } = await seedOrg("p12")
    const created = await createSession(table.id, "OrderBlock")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    // Initiate payment to move to payment_pending
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

    // Try placing a new order — should be blocked
    const orderRes = await fetch(`${API_URL}/sessions/${sessionId}/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({
        idempotency_key: crypto.randomUUID(),
        items: [{ menu_item_id: menu.itemId, quantity: 1 }],
      }),
    })
    expect([409, 422]).toContain(orderRes.status)
  })
})
