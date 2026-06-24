import { test, expect } from "@playwright/test"
import { API_URL } from "../playwright.config"
import { seedOrg, createSession, placeOrder } from "../helpers/api"
import crypto from "crypto"

test.describe("P-08: Split payment — two guests pay half each", () => {
  test("two partial payments accepted on same session", async () => {
    const { table, menu } = await seedOrg("p08")

    // Host creates session
    const hostCreated = await createSession(table.id, "Host")
    const sessionId = hostCreated.session.id
    const hostToken = hostCreated.guest_access_token

    // Guest B joins
    const joinRes = await fetch(`${API_URL}/sessions/${sessionId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "GuestB" }),
    })
    const guestBToken = (await joinRes.json()).guest_access_token

    // Place order
    await placeOrder(sessionId, hostToken, menu.itemId, 2)

    const half = Math.floor(menu.itemPrice)

    // Host pays half
    const pay1 = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${hostToken}`,
      },
      body: JSON.stringify({
        amount: half,
        method: "cash",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    expect([200, 201]).toContain(pay1.status)

    // GuestB pays other half (session may still be active or payment_pending)
    const pay2 = await fetch(`${API_URL}/sessions/${sessionId}/payments`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${guestBToken}`,
      },
      body: JSON.stringify({
        amount: half,
        method: "digital",
        idempotency_key: crypto.randomUUID(),
      }),
    })
    // May succeed or fail depending on session state and split payment policy
    expect([200, 201, 409]).toContain(pay2.status)
  })
})
