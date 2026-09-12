import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder, fetchSnapshot } from "../helpers/api"
import crypto from "crypto"

test.describe("F-02: Cart and order UI frozen during payment_pending", () => {
  // VACUOUS(sig-7): claims frozen UI but uses fetch only; passes when the controls remain enabled.
  test.fixme("snapshot in payment_pending reflects frozen state for frontend", async () => {
    const { table, menu } = await seedOrg("f02")
    const created = await createSession(table.id, "FrozenUI")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    // Initiate payment
    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
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

    const snap = await fetchSnapshot(sessionId, guestToken)
    expect(snap.session.status).toBe("payment_pending")

    // Cart mutations should be blocked
    const cartRes = await fetch(`${API_URL}/sessions/${sessionId}/cart/items`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestToken}`,
      },
      body: JSON.stringify({ menu_item_id: menu.itemId, quantity: 1 }),
    })
    expect([409, 422]).toContain(cartRes.status)
  })
})
