import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-11: Bill snapshot immutable after payment initiated", () => {
  test("bill amounts do not change once payment is in progress", async () => {
    const { table, menu } = await seedOrg("p11")
    const created = await createSession(table.id, "BillSnap")
    const sessionId = created.session.id
    const guestToken = created.guest_access_token

    await placeOrder(sessionId, guestToken, menu.itemId, 1)

    // Fetch bill before payment
    const billBefore = await fetch(`${API_URL}/sessions/${sessionId}/bill`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(billBefore.status).toBe(200)
    const billDataBefore = await billBefore.json()
    const totalBefore = billDataBefore.total ?? billDataBefore.grand_total

    // Initiate payment
    await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
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

    // Fetch bill after payment — total must be identical (immutable)
    const billAfter = await fetch(`${API_URL}/sessions/${sessionId}/bill`, {
      headers: { "Authorization": `Bearer ${guestToken}` },
    })
    expect(billAfter.status).toBe(200)
    const billDataAfter = await billAfter.json()
    const totalAfter = billDataAfter.total ?? billDataAfter.grand_total

    expect(totalAfter).toEqual(totalBefore)
  })
})
